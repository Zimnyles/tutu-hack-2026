package profile

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/tutu-hack/openworld/internal/models/domain"
)

type Service struct {
	repository      Repository
	recommendations PersonalRecommendationStarter
	logger          *slog.Logger
}

func New(
	repository Repository,
	recommendations PersonalRecommendationStarter,
	logger *slog.Logger,
) *Service {
	return &Service{repository: repository, recommendations: recommendations, logger: logger}
}

func (s *Service) SavePreferences(
	ctx context.Context,
	user domain.User,
	preferences domain.Preferences,
) (domain.User, error) {
	normalized, err := normalizePreferences(preferences)
	if err != nil {
		return domain.User{}, err
	}

	user.Preferences = normalized
	if err := s.repository.SaveUser(ctx, user); err != nil {
		return domain.User{}, fmt.Errorf("save user preferences: %w", err)
	}

	if user.OnboardingCompleted {
		if _, err := s.recommendations.RebuildPersonalized(ctx, user); err != nil {
			s.logger.Warn("rebuild personal recommendations", "user_id", user.ID, "error", err)
		}
	}

	return user, nil
}

func (s *Service) SetHomeCity(
	ctx context.Context,
	user domain.User,
	homeCityID string,
) (domain.User, error) {
	normalizedCity, err := s.resolveHomeCity(ctx, homeCityID)
	if err != nil {
		return domain.User{}, err
	}

	if normalizedCity == user.HomeCityID {
		return user, nil
	}

	user.HomeCityID = normalizedCity
	if err := s.repository.SaveUser(ctx, user); err != nil {
		return domain.User{}, fmt.Errorf("save home city: %w", err)
	}

	if user.OnboardingCompleted {
		if _, err := s.recommendations.RebuildPersonalized(ctx, user); err != nil {
			s.logger.Warn("rebuild personal recommendations", "user_id", user.ID, "error", err)
		}
	}

	return user, nil
}

func (s *Service) CompleteOnboarding(
	ctx context.Context,
	user domain.User,
	homeCityID string,
) (domain.User, error) {
	normalizedCity, err := s.resolveHomeCity(ctx, homeCityID)
	if err != nil {
		return domain.User{}, err
	}

	normalizedPreferences, err := normalizePreferences(user.Preferences)
	if err != nil {
		return domain.User{}, err
	}

	user.HomeCityID = normalizedCity
	user.Preferences = normalizedPreferences
	user.OnboardingCompleted = true

	if err := s.repository.SaveUser(ctx, user); err != nil {
		return domain.User{}, fmt.Errorf("complete onboarding: %w", err)
	}

	if _, err := s.recommendations.CreatePersonalized(ctx, user); err != nil {
		s.logger.Warn("start personal recommendations", "user_id", user.ID, "error", err)
	}

	return user, nil
}

func (s *Service) resolveHomeCity(ctx context.Context, homeCityID string) (string, error) {
	normalized, err := normalizeHomeCity(homeCityID)
	if err != nil {
		return "", err
	}

	exists, err := s.repository.CityExists(ctx, normalized)
	if err != nil {
		return "", fmt.Errorf("check home city: %w", err)
	}

	if !exists {
		return "", invalidInput("Такого города нет в списке")
	}

	return normalized, nil
}

func (s *Service) SetTravelVisibility(
	ctx context.Context,
	user domain.User,
	visibility string,
) (domain.User, error) {
	normalized, err := normalizeVisibility(visibility)
	if err != nil {
		return domain.User{}, err
	}

	user.TravelVisibility = normalized
	if err := s.repository.SaveUser(ctx, user); err != nil {
		return domain.User{}, fmt.Errorf("save travel visibility: %w", err)
	}

	return user, nil
}
