package port

import (
	"context"
	"errors"

	"ai-office-backend/internal/core/domain"
)

var (
	ErrUsernameTaken = errors.New("USERNAME_TAKEN")
	ErrUserNotFound  = errors.New("USER_NOT_FOUND")
)

type ConsoleUserRepository interface {
	Create(ctx context.Context, u domain.ConsoleUser) error
	ByUsername(ctx context.Context, username string) (domain.ConsoleUser, error)
	ByID(ctx context.Context, id string) (domain.ConsoleUser, error)
	List(ctx context.Context) ([]domain.ConsoleUser, error)
	Update(ctx context.Context, u domain.ConsoleUser) error
	Delete(ctx context.Context, id string) error
	Count(ctx context.Context) (int, error)
}
