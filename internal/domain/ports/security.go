package ports

import "context"

type PasswordHasher interface {
	Hash(password string) (string, error)
	Compare(hash, password string) error
}

type TokenIssuer interface {
	Issue(userID int64) (string, error)
	Parse(token string) (int64, error)
}

type CacheInvalidator interface {
	InvalidateTaskLists(context.Context, int64) error
}
