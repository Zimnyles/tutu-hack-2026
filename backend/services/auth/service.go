package auth

import (
	"context"
	"fmt"

	"github.com/tutu-hack/openworld/internal/models/domain"
)

type Service struct {
	users              Repository
	sessions           SessionRepository
	transactionManager TransactionManager
}

func New(
	users Repository,
	sessions SessionRepository,
	transactionManager TransactionManager,
) *Service {
	return &Service{
		users:              users,
		sessions:           sessions,
		transactionManager: transactionManager,
	}
}

func (s *Service) Register(
	ctx context.Context,
	email string,
	password string,
	displayName string,
) (domain.User, string, error) {
	var user domain.User

	var sessionToken string

	err := s.transactionManager.WithinTransaction(ctx, func(transactionContext context.Context) error {
		registeredUser, registerErr := s.users.Register(
			transactionContext,
			email,
			password,
			displayName,
		)
		if registerErr != nil {
			return fmt.Errorf("register user: %w", registerErr)
		}

		user = registeredUser

		createdToken, sessionErr := s.sessions.NewSession(transactionContext, user.ID)
		if sessionErr != nil {
			return fmt.Errorf("create registration session: %w", sessionErr)
		}

		sessionToken = createdToken

		return nil
	})
	if err != nil {
		return domain.User{}, "", err
	}

	return user, sessionToken, nil
}

func (s *Service) Login(
	ctx context.Context,
	email string,
	password string,
) (domain.User, string, bool, error) {
	user, authenticated, err := s.users.Authenticate(ctx, email, password)
	if err != nil {
		return domain.User{}, "", false, fmt.Errorf("authenticate user: %w", err)
	}

	if !authenticated {
		return domain.User{}, "", false, nil
	}

	sessionToken, err := s.sessions.NewSession(ctx, user.ID)
	if err != nil {
		return domain.User{}, "", false, fmt.Errorf("create login session: %w", err)
	}

	return user, sessionToken, true, nil
}

func (s *Service) ResolveSession(
	ctx context.Context,
	token string,
) (domain.User, bool, error) {
	user, found, err := s.sessions.Session(ctx, token)
	if err != nil {
		return domain.User{}, false, fmt.Errorf("resolve session: %w", err)
	}

	return user, found, nil
}

func (s *Service) Logout(ctx context.Context, token string) error {
	if err := s.sessions.RevokeSession(ctx, token); err != nil {
		return fmt.Errorf("revoke session: %w", err)
	}

	return nil
}
