package adminsim_storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	adminsim_errors "github.com/tutu-hack/openworld/internal/errors/adminsim"
	"github.com/tutu-hack/openworld/internal/models/domain"
)

type Repository struct {
	database *pgxpool.Pool
}

func NewRepository(database *pgxpool.Pool) *Repository {
	return &Repository{database: database}
}

func (r *Repository) Overview(ctx context.Context) (domain.AdminOverview, error) {
	var overview domain.AdminOverview
	if err := r.database.QueryRow(ctx, `
		SELECT
			TRUE,
			(SELECT COUNT(*) FROM users WHERE is_demo),
			(SELECT COUNT(*) FROM outbox_events WHERE processed_at IS NULL),
			(SELECT COUNT(*) FROM admin_simulation_actions WHERE status = 'failed')
	`).Scan(
		&overview.DatabaseReady,
		&overview.DemoUsers,
		&overview.PendingOutbox,
		&overview.FailedActions,
	); err != nil {
		return domain.AdminOverview{}, fmt.Errorf("query admin overview: %w", err)
	}

	return overview, nil
}

func (r *Repository) Users(
	ctx context.Context,
	query string,
) ([]domain.AdminUserSummary, error) {
	rows, err := r.database.Query(ctx, `
		SELECT
			user_row.id,
			user_row.email,
			user_row.display_name,
			user_row.onboarding_completed_at IS NOT NULL,
			(SELECT COUNT(*) FROM user_visits visit WHERE visit.user_id = user_row.id),
			(SELECT COUNT(*) FROM trips trip WHERE trip.user_id = user_row.id),
			(
				SELECT COALESCE(SUM(reward.amount), 0)::int
				FROM reward_ledger reward
				WHERE reward.user_id = user_row.id
			)
		FROM users user_row
		WHERE user_row.is_demo
		  AND (
			$1 = ''
			OR user_row.email ILIKE '%' || $1 || '%'
			OR user_row.display_name ILIKE '%' || $1 || '%'
		  )
		ORDER BY user_row.created_at DESC
		LIMIT 100
	`, strings.TrimSpace(query))
	if err != nil {
		return nil, fmt.Errorf("query demo users: %w", err)
	}
	defer rows.Close()

	users := make([]domain.AdminUserSummary, 0)

	for rows.Next() {
		var user domain.AdminUserSummary
		if err := rows.Scan(
			&user.ID,
			&user.Email,
			&user.DisplayName,
			&user.OnboardingCompleted,
			&user.Visits,
			&user.Trips,
			&user.RewardBalance,
		); err != nil {
			return nil, fmt.Errorf("scan demo user: %w", err)
		}

		users = append(users, user)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate demo users: %w", err)
	}

	return users, nil
}

func (r *Repository) Scenarios(ctx context.Context) ([]domain.DemoScenario, error) {
	rows, err := r.database.Query(ctx, `
		SELECT id, code, name, description, fixture_version, enabled
		FROM demo_scenarios
		WHERE enabled
		ORDER BY name
	`)
	if err != nil {
		return nil, fmt.Errorf("query demo scenarios: %w", err)
	}
	defer rows.Close()

	scenarios := make([]domain.DemoScenario, 0)

	for rows.Next() {
		var scenario domain.DemoScenario
		if err := rows.Scan(
			&scenario.ID,
			&scenario.Code,
			&scenario.Name,
			&scenario.Description,
			&scenario.FixtureVersion,
			&scenario.Enabled,
		); err != nil {
			return nil, fmt.Errorf("scan demo scenario: %w", err)
		}

		scenarios = append(scenarios, scenario)
	}

	return scenarios, rows.Err()
}

func (r *Repository) AllowedActions(ctx context.Context) ([]string, error) {
	var rawValue []byte
	if err := r.database.QueryRow(ctx, `
		SELECT value FROM app_settings WHERE key = 'admin_action_allowlist'
	`).Scan(&rawValue); err != nil {
		return nil, fmt.Errorf("query admin action allowlist: %w", err)
	}

	var actions []string
	if err := json.Unmarshal(rawValue, &actions); err != nil {
		return nil, fmt.Errorf("decode admin action allowlist: %w", err)
	}

	return actions, nil
}

func (r *Repository) CreateSimulation(
	ctx context.Context,
	command domain.AdminSimulationCommand,
) (domain.AdminSimulation, error) {
	transaction, err := r.database.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return domain.AdminSimulation{}, fmt.Errorf("begin admin simulation: %w", err)
	}

	defer func() {
		_ = transaction.Rollback(ctx)
	}()

	parameters, err := json.Marshal(command.Parameters)
	if err != nil {
		return domain.AdminSimulation{}, fmt.Errorf("encode admin simulation parameters: %w", err)
	}

	var simulation domain.AdminSimulation

	var summary []byte

	err = transaction.QueryRow(ctx, `
		INSERT INTO admin_simulation_actions (
			action_code, actor_user_id, target_type, target_id, demo_scenario_id,
			idempotency_key, reason, request_payload, status, request_id, trace_id
		)
		VALUES (
			$1, $2, $3, NULLIF($4, '')::uuid, NULLIF($5, '')::uuid,
			$6, $7, $8, 'running', NULLIF($9, '')::uuid, $10
		)
		ON CONFLICT (actor_user_id, idempotency_key) DO UPDATE SET
			idempotency_key = EXCLUDED.idempotency_key
		RETURNING id, action_code, target_type, COALESCE(target_id::text, ''), status,
			result_summary, created_at, completed_at
	`,
		command.ActionCode,
		command.ActorUserID,
		command.TargetType,
		command.TargetID,
		command.DemoScenarioID,
		command.IdempotencyKey,
		command.Reason,
		string(parameters),
		command.RequestID,
		command.TraceID,
	).Scan(
		&simulation.ID,
		&simulation.ActionCode,
		&simulation.TargetType,
		&simulation.TargetID,
		&simulation.Status,
		&summary,
		&simulation.CreatedAt,
		&simulation.CompletedAt,
	)
	if err != nil {
		return domain.AdminSimulation{}, fmt.Errorf("insert admin simulation: %w", err)
	}

	if simulation.Status != "running" {
		if err := json.Unmarshal(summary, &simulation.ResultSummary); err != nil {
			return domain.AdminSimulation{}, fmt.Errorf("decode existing simulation: %w", err)
		}

		if err := transaction.Commit(ctx); err != nil {
			return domain.AdminSimulation{}, fmt.Errorf("commit repeated simulation: %w", err)
		}

		return simulation, nil
	}

	result, err := applySimulation(ctx, transaction, command)
	if err != nil {
		return domain.AdminSimulation{}, err
	}

	resultJSON, err := json.Marshal(result)
	if err != nil {
		return domain.AdminSimulation{}, fmt.Errorf("encode simulation result: %w", err)
	}

	if err := transaction.QueryRow(ctx, `
		UPDATE admin_simulation_actions
		SET status = 'completed', result_summary = $2, completed_at = now()
		WHERE id = $1
		RETURNING status, completed_at
	`, simulation.ID, string(resultJSON)).Scan(&simulation.Status, &simulation.CompletedAt); err != nil {
		return domain.AdminSimulation{}, fmt.Errorf("complete admin simulation: %w", err)
	}

	if _, err := transaction.Exec(ctx, `
		INSERT INTO admin_audit_log (
			actor_user_id, action_code, target_type, target_id, outcome,
			reason_code, simulation_action_id, request_id, trace_id, metadata
		)
		VALUES (
			$1, $2, $3, NULLIF($4, '')::uuid, 'success', 'ADMIN_SIMULATION',
			$5, NULLIF($6, '')::uuid, $7, $8
		)
	`,
		command.ActorUserID,
		command.ActionCode,
		command.TargetType,
		command.TargetID,
		simulation.ID,
		command.RequestID,
		command.TraceID,
		string(resultJSON),
	); err != nil {
		return domain.AdminSimulation{}, fmt.Errorf("insert admin audit record: %w", err)
	}

	if err := transaction.Commit(ctx); err != nil {
		return domain.AdminSimulation{}, fmt.Errorf("commit admin simulation: %w", err)
	}

	simulation.ResultSummary = result

	return simulation, nil
}

func (r *Repository) Simulation(
	ctx context.Context,
	simulationID string,
) (domain.AdminSimulation, bool, error) {
	var simulation domain.AdminSimulation

	var summary []byte

	err := r.database.QueryRow(ctx, `
		SELECT id, action_code, target_type, COALESCE(target_id::text, ''), status,
			result_summary, created_at, completed_at
		FROM admin_simulation_actions
		WHERE id = $1
	`, simulationID).Scan(
		&simulation.ID,
		&simulation.ActionCode,
		&simulation.TargetType,
		&simulation.TargetID,
		&simulation.Status,
		&summary,
		&simulation.CreatedAt,
		&simulation.CompletedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.AdminSimulation{}, false, nil
	}

	if err != nil {
		return domain.AdminSimulation{}, false, fmt.Errorf("query admin simulation: %w", err)
	}

	if err := json.Unmarshal(summary, &simulation.ResultSummary); err != nil {
		return domain.AdminSimulation{}, false, fmt.Errorf("decode admin simulation: %w", err)
	}

	return simulation, true, nil
}

func (r *Repository) AuditLog(ctx context.Context) ([]domain.AdminAuditEntry, error) {
	rows, err := r.database.Query(ctx, `
		SELECT id, action_code, target_type, COALESCE(target_id::text, ''),
			outcome, reason_code, metadata, created_at
		FROM admin_audit_log
		ORDER BY created_at DESC
		LIMIT 200
	`)
	if err != nil {
		return nil, fmt.Errorf("query admin audit log: %w", err)
	}
	defer rows.Close()

	entries := make([]domain.AdminAuditEntry, 0)

	for rows.Next() {
		var entry domain.AdminAuditEntry

		var metadata []byte
		if err := rows.Scan(
			&entry.ID,
			&entry.ActionCode,
			&entry.TargetType,
			&entry.TargetID,
			&entry.Outcome,
			&entry.ReasonCode,
			&metadata,
			&entry.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan admin audit record: %w", err)
		}

		if err := json.Unmarshal(metadata, &entry.Metadata); err != nil {
			return nil, fmt.Errorf("decode admin audit metadata: %w", err)
		}

		entries = append(entries, entry)
	}

	return entries, rows.Err()
}

func applySimulation(
	ctx context.Context,
	transaction pgx.Tx,
	command domain.AdminSimulationCommand,
) (map[string]any, error) {
	if err := assertDemoTarget(ctx, transaction, command); err != nil {
		return nil, err
	}

	var commandTag pgconnCommandTag

	var err error

	switch command.ActionCode {
	case "demo_sync_history":
		commandTag, err = transaction.Exec(ctx, `
			INSERT INTO user_visits (user_id, territory_id, source, level, visited_at, evidence_ref)
			SELECT $1, history.destination_city_id, 'demo_sync', 1, history.arrived_at, history.external_order_ref
			FROM demo_travel_history history
			JOIN users target_user ON target_user.id = $1 AND target_user.is_demo
			ON CONFLICT (user_id, territory_id, source) DO NOTHING
		`, command.TargetID)
	case "trip_checkout_created", "trip_departed", "trip_cancelled":
		status := strings.TrimPrefix(command.ActionCode, "trip_")
		commandTag, err = transaction.Exec(ctx, `
			UPDATE trips SET status = $2, updated_at = now()
			WHERE id = $1 AND is_demo
		`, command.TargetID, status)
	case "event_cancel":
		commandTag, err = transaction.Exec(ctx, `
			UPDATE events SET status = 'cancelled', updated_at = now()
			WHERE id = $1 AND is_demo
		`, command.TargetID)
	case "event_set_availability":
		availability, _ := command.Parameters["availability"].(string)
		commandTag, err = transaction.Exec(ctx, `
			UPDATE events SET availability = $2, updated_at = now()
			WHERE id = $1 AND is_demo
		`, command.TargetID, availability)
	case "cohort_set_demo_size":
		count, _ := command.Parameters["count"].(float64)
		guildCount, _ := command.Parameters["guild_count"].(float64)
		commandTag, err = transaction.Exec(ctx, `
			UPDATE travel_cohorts
			SET demo_aggregate_count = $2, demo_guild_count = $3
			WHERE id = $1
		`, command.TargetID, int(count), int(guildCount))
	case "outbox_process":
		commandTag, err = transaction.Exec(ctx, `
			UPDATE outbox_events SET processed_at = now(), attempts = attempts + 1
			WHERE processed_at IS NULL
		`)
	case "leaderboard_rebuild":
		commandTag, err = transaction.Exec(ctx, `
			UPDATE leaderboard_snapshots SET generated_at = now()
		`)
	case "demo_profile_reset":
		commandTag, err = transaction.Exec(ctx, `
			DELETE FROM user_visits
			WHERE user_id = $1 AND EXISTS (
				SELECT 1 FROM users WHERE id = $1 AND is_demo
			)
		`, command.TargetID)
	default:
		commandTag, err = transaction.Exec(ctx, `SELECT 1`)
	}

	if err != nil {
		return nil, fmt.Errorf("apply admin action %s: %w", command.ActionCode, err)
	}

	return map[string]any{
		"action_code":   command.ActionCode,
		"affected_rows": commandTag.RowsAffected(),
		"demo_only":     true,
	}, nil
}

type pgconnCommandTag interface {
	RowsAffected() int64
}

func assertDemoTarget(
	ctx context.Context,
	transaction pgx.Tx,
	command domain.AdminSimulationCommand,
) error {
	if command.TargetID == "" {
		return nil
	}

	var isDemo bool

	var err error

	switch command.TargetType {
	case "user":
		err = transaction.QueryRow(ctx, `SELECT is_demo FROM users WHERE id = $1`, command.TargetID).Scan(&isDemo)
	case "trip":
		err = transaction.QueryRow(ctx, `SELECT is_demo FROM trips WHERE id = $1`, command.TargetID).Scan(&isDemo)
	case "event":
		err = transaction.QueryRow(ctx, `SELECT is_demo FROM events WHERE id = $1`, command.TargetID).Scan(&isDemo)
	default:
		return nil
	}

	if errors.Is(err, pgx.ErrNoRows) || !isDemo {
		return adminsim_errors.ErrTargetNotDemo
	}

	if err != nil {
		return fmt.Errorf("validate demo target: %w", err)
	}

	return nil
}
