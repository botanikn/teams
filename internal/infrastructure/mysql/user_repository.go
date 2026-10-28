package mysql

import (
	"context"
	"database/sql"
	"errors"

	"task-api/internal/domain/entities"
	"task-api/internal/domain/errrors"
)

type UserRepository struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(ctx context.Context, user *entities.User) error {
	result, err := r.db.ExecContext(ctx, `INSERT INTO users (email, password_hash, name) VALUES (?, ?, ?)`, user.Email, user.PasswordHash, user.Name)
	if err != nil {
		return mapDBError(err)
	}
	user.ID, err = result.LastInsertId()
	return err
}

func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*entities.User, error) {
	return scanUser(r.db.QueryRowContext(ctx, `SELECT id, email, password_hash, name, created_at FROM users WHERE email = ?`, email))
}

func (r *UserRepository) FindByID(ctx context.Context, id int64) (*entities.User, error) {
	return scanUser(r.db.QueryRowContext(ctx, `SELECT id, email, password_hash, name, created_at FROM users WHERE id = ?`, id))
}

func scanUser(row *sql.Row) (*entities.User, error) {
	user := new(entities.User)
	err := row.Scan(&user.ID, &user.Email, &user.PasswordHash, &user.Name, &user.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errrors.ErrNotFound
	}
	return user, err
}
