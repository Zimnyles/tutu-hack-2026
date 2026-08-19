package adminsim

import (
	"context"

	"github.com/tutu-hack/openworld/internal/models/domain"
)

type QueryRepository interface {
	Overview(context.Context) (domain.AdminOverview, error)
	Users(context.Context, string) ([]domain.AdminUserSummary, error)
	Scenarios(context.Context) ([]domain.DemoScenario, error)
	Simulation(context.Context, string) (domain.AdminSimulation, bool, error)
	AuditLog(context.Context) ([]domain.AdminAuditEntry, error)
}

type CommandRepository interface {
	AllowedActions(context.Context) ([]string, error)
	CreateSimulation(context.Context, domain.AdminSimulationCommand) (domain.AdminSimulation, error)
}
