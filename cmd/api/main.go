package main

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"github.com/redis/go-redis/v9"
	"task-api/internal/application"
	"task-api/internal/config"
	mysqlinfra "task-api/internal/infrastructure/mysql"
	rediscache "task-api/internal/infrastructure/redis"
	"task-api/internal/infrastructure/security"
	"task-api/internal/presentation/httpapi"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	mysqlDriver.SetLogger(log.New(os.Stderr, "mysql: ", log.LstdFlags))
	db, err := sql.Open("mysql", cfg.MySQLDSN)
	if err != nil {
		return err
	}
	defer db.Close()
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(5 * time.Minute)

	startupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := db.PingContext(startupCtx); err != nil {
		return err
	}
	if err := mysqlinfra.Migrate(startupCtx, db); err != nil {
		return err
	}

	redisClient := redis.NewClient(&redis.Options{
		Addr: cfg.RedisAddr, Password: cfg.RedisPassword, DB: cfg.RedisDB,
	})
	defer redisClient.Close()
	if err := redisClient.Ping(startupCtx).Err(); err != nil {
		return err
	}

	repository := mysqlinfra.NewRepository(db)
	userRepository := mysqlinfra.NewUserRepository(db)
	tokens := security.NewJWTIssuer(cfg.JWTSecret, cfg.JWTTTL)
	cachedTasks := rediscache.NewCachedTaskRepository(repository, repository, repository, repository, redisClient)
	authService := application.NewAuthService(userRepository, security.BcryptHasher{}, tokens)
	teamService := application.NewTeamService(repository, userRepository)
	taskService := application.NewTaskService(cachedTasks, cachedTasks, cachedTasks, cachedTasks, repository)
	handler := httpapi.NewHandler(authService, teamService, taskService, tokens)

	server := &http.Server{
		Addr: cfg.HTTPAddr, Handler: handler.Routes(),
		ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second,
		WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second,
	}
	serverErrors := make(chan error, 1)
	go func() {
		slog.Info("HTTP server listening", "address", cfg.HTTPAddr)
		serverErrors <- server.ListenAndServe()
	}()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	select {
	case err := <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-signals:
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			return err
		}
	}
	return nil
}
