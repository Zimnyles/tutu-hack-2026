package adminsim_handler

import (
	"context"

	"github.com/tutu-hack/openworld/internal/models/domain"
)

type AdminSimulationService interface {
	Overview(context.Context, domain.User) (domain.AdminOverview, error)
	Users(context.Context, domain.User, string) ([]domain.AdminUserSummary, error)
	Scenarios(context.Context, domain.User) ([]domain.DemoScenario, error)
	Execute(context.Context, domain.User, domain.AdminSimulationCommand) (domain.AdminSimulation, error)
	Simulation(context.Context, domain.User, string) (domain.AdminSimulation, bool, error)
	AuditLog(context.Context, domain.User) ([]domain.AdminAuditEntry, error)
}
