package resources

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

const (
	connectionCheckTimeout = 5 * time.Second
	connectionMaxLifetime  = 30 * time.Minute
	connectionMaxIdleTime  = 5 * time.Minute
	connectionHealthPeriod = time.Minute
	redisDialTimeout       = 3 * time.Second
	redisOperationTimeout  = 2 * time.Second
)

type Resources struct {
	Env      *Env
	Logger   *slog.Logger
	Database *pgxpool.Pool
	Redis    *redis.Client
}

func InitResources(ctx context.Context) (*Resources, error) {
	env, err := LoadEnv()
	if err != nil {
		return nil, err
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: env.LogLevel}))

	database, err := initDatabase(ctx, env)
	if err != nil {
		return nil, err
	}

	redisClient, err := initRedis(ctx, env)
	if err != nil {
		database.Close()

		return nil, err
	}

	return &Resources{
		Env:      env,
		Logger:   logger,
		Database: database,
		Redis:    redisClient,
	}, nil
}

func initDatabase(ctx context.Context, env *Env) (*pgxpool.Pool, error) {
	databaseConfig, err := pgxpool.ParseConfig(env.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database config: %w", err)
	}

	databaseConfig.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
	databaseConfig.MaxConns = int32(env.DatabaseMaxConnections)
	databaseConfig.MinConns = 1
	databaseConfig.MaxConnLifetime = connectionMaxLifetime
	databaseConfig.MaxConnIdleTime = connectionMaxIdleTime
	databaseConfig.HealthCheckPeriod = connectionHealthPeriod

	database, err := pgxpool.NewWithConfig(ctx, databaseConfig)
	if err != nil {
		return nil, fmt.Errorf("create database pool: %w", err)
	}

	pingContext, cancel := context.WithTimeout(ctx, connectionCheckTimeout)
	defer cancel()

	if err := database.Ping(pingContext); err != nil {
		database.Close()

		return nil, fmt.Errorf("ping database: %w", err)
	}

	return database, nil
}

func initRedis(ctx context.Context, env *Env) (*redis.Client, error) {
	redisClient := redis.NewClient(&redis.Options{
		Addr:         env.RedisAddress,
		Password:     env.RedisPassword,
		DialTimeout:  redisDialTimeout,
		ReadTimeout:  redisOperationTimeout,
		WriteTimeout: redisOperationTimeout,
		PoolSize:     env.DatabaseMaxConnections,
	})

	pingContext, cancel := context.WithTimeout(ctx, connectionCheckTimeout)
	defer cancel()

	if err := redisClient.Ping(pingContext).Err(); err != nil {
		_ = redisClient.Close()

		return nil, fmt.Errorf("ping Redis: %w", err)
	}

	return redisClient, nil
}

func (r *Resources) Close() {
	if r.Database != nil {
		r.Database.Close()
	}

	if r.Redis != nil {
		_ = r.Redis.Close()
	}
}

func (r *Resources) Check(ctx context.Context) error {
	checkContext, cancel := context.WithTimeout(ctx, connectionCheckTimeout)
	defer cancel()

	if err := r.Database.Ping(checkContext); err != nil {
		return fmt.Errorf("check PostgreSQL: %w", err)
	}

	if err := r.Redis.Ping(checkContext).Err(); err != nil {
		return fmt.Errorf("check Redis: %w", err)
	}

	return nil
}
