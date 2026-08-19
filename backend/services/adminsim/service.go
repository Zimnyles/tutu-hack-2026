package adminsim

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	adminsim_errors "github.com/tutu-hack/openworld/internal/errors/adminsim"
	"github.com/tutu-hack/openworld/internal/models/domain"
)

const (
	minimumReasonLength         = 12
	maximumReasonLength         = 500
	minimumIdempotencyKeyLength = 8
	maximumIdempotencyKeyLength = 128
	maximumParameters           = 20
	maximumSearchQueryLength    = 120
)

var allowedTargetTypes = map[string]struct{}{ //nolint:gochecknoglobals // static allowlist.
	"user":     {},
	"trip":     {},
	"season":   {},
	"guild":    {},
	"scenario": {},
	"system":   {},
}

type Service struct {
	queries  QueryRepository
	commands CommandRepository
	enabled  bool
}

func New(
	queries QueryRepository,
	commands CommandRepository,
	enabled bool,
) *Service {
	return &Service{
		queries:  queries,
		commands: commands,
		enabled:  enabled,
	}
}

func (s *Service) Overview(
	ctx context.Context,
	actor domain.User,
) (domain.AdminOverview, error) {
	if err := s.authorize(actor); err != nil {
		return domain.AdminOverview{}, err
	}

	overview, err := s.queries.Overview(ctx)
	if err != nil {
		return domain.AdminOverview{}, fmt.Errorf("load admin overview: %w", err)
	}

	allowedActions, err := s.commands.AllowedActions(ctx)
	if err != nil {
		return domain.AdminOverview{}, fmt.Errorf("load admin action allowlist: %w", err)
	}

	overview.SimulatorEnabled = s.enabled
	overview.AvailableActions = allowedActions

	return overview, nil
}

func (s *Service) Users(
	ctx context.Context,
	actor domain.User,
	query string,
) ([]domain.AdminUserSummary, error) {
	if err := s.authorize(actor); err != nil {
		return nil, err
	}

	trimmedQuery := strings.TrimSpace(query)
	if utf8.RuneCountInString(trimmedQuery) > maximumSearchQueryLength {
		return nil, adminsim_errors.ErrInvalidQuery
	}

	users, err := s.queries.Users(ctx, trimmedQuery)
	if err != nil {
		return nil, fmt.Errorf("search admin users: %w", err)
	}

	return users, nil
}

func (s *Service) Scenarios(
	ctx context.Context,
	actor domain.User,
) ([]domain.DemoScenario, error) {
	if err := s.authorize(actor); err != nil {
		return nil, err
	}

	scenarios, err := s.queries.Scenarios(ctx)
	if err != nil {
		return nil, fmt.Errorf("load demo scenarios: %w", err)
	}

	return scenarios, nil
}

func (s *Service) Execute(
	ctx context.Context,
	actor domain.User,
	command domain.AdminSimulationCommand,
) (domain.AdminSimulation, error) {
	if err := s.authorize(actor); err != nil {
		return domain.AdminSimulation{}, err
	}

	command, err := normalizeCommand(command)
	if err != nil {
		return domain.AdminSimulation{}, err
	}

	allowedActions, err := s.commands.AllowedActions(ctx)
	if err != nil {
		return domain.AdminSimulation{}, fmt.Errorf("load admin action allowlist: %w", err)
	}

	if !contains(allowedActions, command.ActionCode) {
		return domain.AdminSimulation{}, adminsim_errors.ErrActionNotAllowed
	}

	command.ActorUserID = actor.ID

	simulation, err := s.commands.CreateSimulation(ctx, command)
	if err != nil {
		return domain.AdminSimulation{}, fmt.Errorf("execute admin simulation: %w", err)
	}

	return simulation, nil
}

func normalizeCommand(
	command domain.AdminSimulationCommand,
) (domain.AdminSimulationCommand, error) {
	command.Reason = strings.TrimSpace(command.Reason)

	reasonLength := utf8.RuneCountInString(command.Reason)
	if reasonLength < minimumReasonLength || reasonLength > maximumReasonLength {
		return domain.AdminSimulationCommand{}, adminsim_errors.ErrInvalidReason
	}

	command.IdempotencyKey = strings.TrimSpace(command.IdempotencyKey)
	if len(command.IdempotencyKey) < minimumIdempotencyKeyLength ||
		len(command.IdempotencyKey) > maximumIdempotencyKeyLength {
		return domain.AdminSimulationCommand{}, adminsim_errors.ErrInvalidIdempotencyKey
	}

	command.TargetType = strings.ToLower(strings.TrimSpace(command.TargetType))
	if _, allowed := allowedTargetTypes[command.TargetType]; !allowed {
		return domain.AdminSimulationCommand{}, adminsim_errors.ErrInvalidTarget
	}

	targetID, err := normalizeOptionalIdentifier(command.TargetID)
	if err != nil {
		return domain.AdminSimulationCommand{}, err
	}

	command.TargetID = targetID

	scenarioID, err := normalizeOptionalIdentifier(command.DemoScenarioID)
	if err != nil {
		return domain.AdminSimulationCommand{}, err
	}

	command.DemoScenarioID = scenarioID

	if len(command.Parameters) > maximumParameters {
		return domain.AdminSimulationCommand{}, adminsim_errors.ErrInvalidParameters
	}

	return command, nil
}

func normalizeOptionalIdentifier(rawValue string) (string, error) {
	trimmed := strings.TrimSpace(rawValue)
	if trimmed == "" {
		return "", nil
	}

	parsed, err := uuid.Parse(trimmed)
	if err != nil {
		return "", adminsim_errors.ErrInvalidTarget
	}

	return parsed.String(), nil
}

func (s *Service) Simulation(
	ctx context.Context,
	actor domain.User,
	simulationID string,
) (domain.AdminSimulation, bool, error) {
	if err := s.authorize(actor); err != nil {
		return domain.AdminSimulation{}, false, err
	}

	if _, err := uuid.Parse(strings.TrimSpace(simulationID)); err != nil {
		return domain.AdminSimulation{}, false, adminsim_errors.ErrInvalidTarget
	}

	simulation, found, err := s.queries.Simulation(ctx, simulationID)
	if err != nil {
		return domain.AdminSimulation{}, false, fmt.Errorf("load admin simulation: %w", err)
	}

	return simulation, found, nil
}

func (s *Service) AuditLog(
	ctx context.Context,
	actor domain.User,
) ([]domain.AdminAuditEntry, error) {
	if err := s.authorize(actor); err != nil {
		return nil, err
	}

	entries, err := s.queries.AuditLog(ctx)
	if err != nil {
		return nil, fmt.Errorf("load admin audit log: %w", err)
	}

	return entries, nil
}

func (s *Service) authorize(actor domain.User) error {
	if !s.enabled || actor.Role != "demo_admin" {
		return adminsim_errors.ErrAccessDenied
	}

	return nil
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}

	return false
}
