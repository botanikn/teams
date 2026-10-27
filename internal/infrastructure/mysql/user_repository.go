package mysql

import (
	"context"
	"database/sql"
	"errors"

	"task-api/internal/domain"
)

type UserRepository struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(ctx context.Context, user *domain.User) error {
	result, err := r.db.ExecContext(ctx, `INSERT INTO users (email, password_hash, name) VALUES (?, ?, ?)`, user.Email, user.PasswordHash, user.Name)
	if err != nil {
		return mapDBError(err)
	}
	user.ID, err = result.LastInsertId()
	return err
}

func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	return scanUser(r.db.QueryRowContext(ctx, `SELECT id, email, password_hash, name, created_at FROM users WHERE email = ?`, email))
}

func (r *UserRepository) FindByID(ctx context.Context, id int64) (*domain.User, error) {
	return scanUser(r.db.QueryRowContext(ctx, `SELECT id, email, password_hash, name, created_at FROM users WHERE id = ?`, id))
}

func scanUser(row *sql.Row) (*domain.User, error) {
	user := new(domain.User)
	err := row.Scan(&user.ID, &user.Email, &user.PasswordHash, &user.Name, &user.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	return user, err
}
