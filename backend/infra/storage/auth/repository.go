package auth_storage

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tutu-hack/openworld/infra/storage/postgres"
	auth_errors "github.com/tutu-hack/openworld/internal/errors/auth"
	common_errors "github.com/tutu-hack/openworld/internal/errors/common"
	"github.com/tutu-hack/openworld/internal/models/domain"
	"github.com/tutu-hack/openworld/internal/security"
)

const (
	sessionTokenBytes = 32
	defaultHomeCity   = "yekaterinburg"
)

type Repository struct {
	database        *pgxpool.Pool
	sessionLifetime time.Duration
}

func NewRepository(database *pgxpool.Pool, sessionLifetime time.Duration) *Repository {
	return &Repository{database: database, sessionLifetime: sessionLifetime}
}

func (r *Repository) Register(
	ctx context.Context,
	email string,
	password string,
	displayName string,
) (domain.User, error) {
	passwordHash, err := security.HashPassword(password)
	if err != nil {
		return domain.User{}, fmt.Errorf("hash password: %w", err)
	}

	normalizedEmail := strings.ToLower(strings.TrimSpace(email))
	normalizedName := strings.TrimSpace(displayName)
	executor := postgres.ExecutorFromContext(ctx, r.database)

	var userID string

	err = executor.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, display_name, home_city_id)
		VALUES (
			$1,
			$2,
			$3,
			(SELECT id FROM territories WHERE slug = $4 LIMIT 1)
		)
		RETURNING id
	`, normalizedEmail, passwordHash, normalizedName, defaultHomeCity).Scan(&userID)
	if err != nil {
		if postgres.IsUniqueViolation(err) {
			return domain.User{}, auth_errors.ErrCredentials
		}

		return domain.User{}, fmt.Errorf("insert user: %w", err)
	}

	if _, err := executor.Exec(
		ctx,
		`INSERT INTO user_preferences (user_id) VALUES ($1)`,
		userID,
	); err != nil {
		return domain.User{}, fmt.Errorf("insert user preferences: %w", err)
	}

	user, found, err := r.userByID(ctx, userID)
	if err != nil {
		return domain.User{}, err
	}

	if !found {
		return domain.User{}, common_errors.ErrNotFound
	}

	return user, nil
}

func (r *Repository) Authenticate(
	ctx context.Context,
	email string,
	password string,
) (domain.User, bool, error) {
	normalizedEmail := strings.ToLower(strings.TrimSpace(email))
	executor := postgres.ExecutorFromContext(ctx, r.database)

	row := executor.QueryRow(ctx, userSelect+` WHERE user_row.email = $1`, normalizedEmail)

	user, passwordHash, err := scanUserWithPassword(row)
	if postgres.IsNotFound(err) {
		security.EqualizeVerificationCost(password)

		return domain.User{}, false, nil
	}

	if err != nil {
		return domain.User{}, false, fmt.Errorf("query authenticated user: %w", err)
	}

	if !security.VerifyPassword(passwordHash, password) {
		return domain.User{}, false, nil
	}

	return user, true, nil
}

func (r *Repository) NewSession(ctx context.Context, userID string) (string, error) {
	rawToken, tokenHash, err := generateSessionToken()
	if err != nil {
		return "", err
	}

	executor := postgres.ExecutorFromContext(ctx, r.database)
	if _, err := executor.Exec(ctx, `
		INSERT INTO sessions (user_id, token_hash, expires_at)
		VALUES ($1, $2, $3)
	`, userID, tokenHash, time.Now().Add(r.sessionLifetime)); err != nil {
		return "", fmt.Errorf("insert session: %w", err)
	}

	return rawToken, nil
}

func (r *Repository) Session(
	ctx context.Context,
	rawToken string,
) (domain.User, bool, error) {
	if rawToken == "" {
		return domain.User{}, false, nil
	}

	executor := postgres.ExecutorFromContext(ctx, r.database)

	user, err := scanUser(executor.QueryRow(ctx, `
		SELECT
			user_row.id,
			user_row.email,
			user_row.display_name,
			COALESCE(user_row.home_city_id::text, ''),
			user_row.role,
			user_row.is_demo,
			user_row.onboarding_completed_at IS NOT NULL,
			user_row.travel_visibility,
			preference.themes,
			preference.transport_modes,
			preference.max_travel_minutes,
			preference.typical_budget,
			preference.trip_duration_days,
			preference.spontaneity,
			preference.avoid
		FROM sessions session_row
		JOIN users user_row ON user_row.id = session_row.user_id
		JOIN user_preferences preference ON preference.user_id = user_row.id
		WHERE session_row.token_hash = $1
		  AND session_row.revoked_at IS NULL
		  AND session_row.expires_at > now()
	`, hashSessionToken(rawToken)))
	if postgres.IsNotFound(err) {
		return domain.User{}, false, nil
	}

	if err != nil {
		return domain.User{}, false, fmt.Errorf("query session: %w", err)
	}

	return user, true, nil
}

func (r *Repository) RevokeSession(ctx context.Context, rawToken string) error {
	executor := postgres.ExecutorFromContext(ctx, r.database)
	if _, err := executor.Exec(ctx, `
		UPDATE sessions
		SET revoked_at = now()
		WHERE token_hash = $1 AND revoked_at IS NULL
	`, hashSessionToken(rawToken)); err != nil {
		return fmt.Errorf("revoke session: %w", err)
	}

	return nil
}

func (r *Repository) userByID(
	ctx context.Context,
	userID string,
) (domain.User, bool, error) {
	executor := postgres.ExecutorFromContext(ctx, r.database)

	user, err := scanUser(executor.QueryRow(ctx, userSelectWithoutPassword+`
		WHERE user_row.id = $1
	`, userID))
	if postgres.IsNotFound(err) {
		return domain.User{}, false, nil
	}

	if err != nil {
		return domain.User{}, false, fmt.Errorf("query user: %w", err)
	}

	return user, true, nil
}

type rowScanner interface {
	Scan(...any) error
}

func scanUser(row rowScanner) (domain.User, error) {
	var user domain.User
	err := row.Scan(
		&user.ID,
		&user.Email,
		&user.DisplayName,
		&user.HomeCityID,
		&user.Role,
		&user.Demo,
		&user.OnboardingCompleted,
		&user.TravelVisibility,
		&user.Preferences.Themes,
		&user.Preferences.TransportModes,
		&user.Preferences.MaxTravelMinutes,
		&user.Preferences.TypicalBudget,
		&user.Preferences.TripDurationDays,
		&user.Preferences.Spontaneity,
		&user.Preferences.Avoid,
	)

	return user, err
}

func scanUserWithPassword(row rowScanner) (domain.User, string, error) {
	var user domain.User

	var passwordHash string

	err := row.Scan(
		&user.ID,
		&user.Email,
		&user.DisplayName,
		&user.HomeCityID,
		&user.Role,
		&user.Demo,
		&user.OnboardingCompleted,
		&user.TravelVisibility,
		&user.Preferences.Themes,
		&user.Preferences.TransportModes,
		&user.Preferences.MaxTravelMinutes,
		&user.Preferences.TypicalBudget,
		&user.Preferences.TripDurationDays,
		&user.Preferences.Spontaneity,
		&user.Preferences.Avoid,
		&passwordHash,
	)

	return user, passwordHash, err
}

func generateSessionToken() (string, []byte, error) {
	randomBytes := make([]byte, sessionTokenBytes)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", nil, fmt.Errorf("generate session token: %w", err)
	}

	rawToken := hex.EncodeToString(randomBytes)
	tokenHash := sha256.Sum256([]byte(rawToken))

	return rawToken, tokenHash[:], nil
}

func hashSessionToken(rawToken string) []byte {
	tokenHash := sha256.Sum256([]byte(rawToken))

	return tokenHash[:]
}

const userSelect = userSelectColumns + `, user_row.password_hash
	FROM users user_row
	JOIN user_preferences preference ON preference.user_id = user_row.id`

const userSelectWithoutPassword = userSelectColumns + `
	FROM users user_row
	JOIN user_preferences preference ON preference.user_id = user_row.id`

const userSelectColumns = `
	SELECT
		user_row.id,
		user_row.email,
		user_row.display_name,
		COALESCE(user_row.home_city_id::text, ''),
		user_row.role,
		user_row.is_demo,
		user_row.onboarding_completed_at IS NOT NULL,
		user_row.travel_visibility,
		preference.themes,
		preference.transport_modes,
		preference.max_travel_minutes,
		preference.typical_budget,
		preference.trip_duration_days,
		preference.spontaneity,
		preference.avoid`
