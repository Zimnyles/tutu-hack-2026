package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	bootstrap_storage "github.com/tutu-hack/openworld/infra/storage/bootstrap"
	"github.com/tutu-hack/openworld/infra/storage/postgres"
	"github.com/tutu-hack/openworld/internal/api"
	"github.com/tutu-hack/openworld/resources"
)

func main() {
	if err := run(); err != nil {
		slog.Error("application stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(
		context.Background(),
		syscall.SIGINT,
		syscall.SIGTERM,
	)
	defer stop()

	applicationResources, err := resources.InitResources(ctx)
	if err != nil {
		return err
	}
	defer applicationResources.Close()

	if err := postgres.Migrate(ctx, applicationResources.Database); err != nil {
		return err
	}

	if applicationResources.Env.DemoMode {
		if err := prepareDemoData(ctx, applicationResources); err != nil {
			return err
		}
	}

	return api.New(applicationResources).Start(ctx)
}

func prepareDemoData(ctx context.Context, appResources *resources.Resources) error {
	if err := postgres.SeedFixtures(ctx, appResources.Database); err != nil {
		return err
	}

	bootstrapRepository := bootstrap_storage.NewRepository(appResources.Database)
	if err := bootstrapRepository.EnsureDemoAccount(
		ctx,
		appResources.Env.DemoUserEmail,
		appResources.Env.DemoUserPassword,
		appResources.Env.DemoUserName,
		"user",
	); err != nil {
		return err
	}

	return bootstrapRepository.EnsureDemoAccount(
		ctx,
		appResources.Env.DemoAdminEmail,
		appResources.Env.DemoAdminPassword,
		appResources.Env.DemoAdminName,
		"demo_admin",
	)
}
