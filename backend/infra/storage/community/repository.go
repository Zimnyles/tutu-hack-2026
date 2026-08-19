package community_storage

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tutu-hack/openworld/infra/storage/postgres"
	community_errors "github.com/tutu-hack/openworld/internal/errors/community"
	"github.com/tutu-hack/openworld/internal/models/domain"
)

type Repository struct {
	database *pgxpool.Pool
}

type leagueRule struct {
	MinimumScore int    `json:"minimum_score"`
	Name         string `json:"name"`
	NextScore    int    `json:"next_score"`
	Percentile   int    `json:"percentile"`
}

func NewRepository(database *pgxpool.Pool) *Repository {
	return &Repository{database: database}
}

func (r *Repository) CurrentSeason(
	ctx context.Context,
	userID string,
) (domain.Season, error) {
	var season domain.Season
	if err := r.database.QueryRow(ctx, `
		SELECT
			season.id,
			season.name,
			season.month_title,
			season.status,
			season.starts_at,
			season.ends_at,
			season.rules_version,
			COALESCE(SUM(score.points), 0)::int
		FROM seasons season
		LEFT JOIN season_score_ledger score
			ON score.season_id = season.id AND score.user_id = $1
		WHERE season.status = 'active'
		GROUP BY season.id
		ORDER BY season.starts_at DESC
		LIMIT 1
	`, userID).Scan(
		&season.ID,
		&season.Name,
		&season.MonthTitle,
		&season.Status,
		&season.StartsAt,
		&season.EndsAt,
		&season.RulesVersion,
		&season.UserScore,
	); err != nil {
		return domain.Season{}, fmt.Errorf("query current season: %w", err)
	}

	rules, err := r.leagueRules(ctx)
	if err != nil {
		return domain.Season{}, err
	}

	season.League, season.NextLeagueScore, season.Percentile, err = leagueForScore(season.UserScore, rules)
	if err != nil {
		return domain.Season{}, err
	}

	return season, nil
}

func (r *Repository) Leaderboard(
	ctx context.Context,
	scope string,
	period string,
) (domain.Leaderboard, error) {
	var leaderboard domain.Leaderboard

	var payload []byte

	err := r.database.QueryRow(ctx, `
		SELECT scope, period, generated_at, payload
		FROM leaderboard_snapshots
		WHERE scope = $1 AND period = $2
		ORDER BY generated_at DESC
		LIMIT 1
	`, scope, period).Scan(
		&leaderboard.Scope,
		&leaderboard.Period,
		&leaderboard.GeneratedAt,
		&payload,
	)
	if postgres.IsNotFound(err) {
		return domain.Leaderboard{
			Scope:  scope,
			Period: period,
			Items:  []domain.LeaderboardItem{},
		}, nil
	}

	if err != nil {
		return domain.Leaderboard{}, fmt.Errorf("query leaderboard snapshot: %w", err)
	}

	if err := json.Unmarshal(payload, &leaderboard.Items); err != nil {
		return domain.Leaderboard{}, fmt.Errorf("decode leaderboard snapshot: %w", err)
	}

	return leaderboard, nil
}

func (r *Repository) SuggestedGuild(
	ctx context.Context,
	userID string,
) (domain.Guild, error) {
	var guild domain.Guild
	if err := r.database.QueryRow(ctx, `
		SELECT
			guild.id,
			guild.name,
			guild.territory_id,
			guild.emblem_asset,
			guild.level,
			guild.demo_member_count + COALESCE(member_stats.members, 0),
			guild.demo_base_score + COALESCE(score_stats.points, 0),
			guild.demo_rank,
			EXISTS (
				SELECT 1 FROM guild_memberships own_membership
				WHERE own_membership.guild_id = guild.id
				  AND own_membership.user_id = $1
				  AND own_membership.left_at IS NULL
			),
			COALESCE((
				SELECT SUM(own_score.points)
				FROM season_score_ledger own_score
				WHERE own_score.user_id = $1 AND own_score.guild_id = guild.id
			), 0),
			challenge.title,
			challenge.description,
			challenge.demo_base_progress + COALESCE(score_stats.entries, 0),
			challenge.target_value
		FROM users user_row
		JOIN guilds guild ON guild.territory_id = user_row.home_city_id
		JOIN guild_challenges challenge ON challenge.guild_id = guild.id AND challenge.status = 'active'
		LEFT JOIN LATERAL (
			SELECT COUNT(*)::int AS members
			FROM guild_memberships membership
			WHERE membership.guild_id = guild.id AND membership.left_at IS NULL
		) member_stats ON TRUE
		LEFT JOIN LATERAL (
			SELECT COALESCE(SUM(score.points), 0)::int AS points, COUNT(*)::int AS entries
			FROM season_score_ledger score
			WHERE score.guild_id = guild.id
		) score_stats ON TRUE
		WHERE user_row.id = $1
	`, userID).Scan(
		&guild.ID,
		&guild.Name,
		&guild.CityID,
		&guild.Emblem,
		&guild.Level,
		&guild.Members,
		&guild.SeasonScore,
		&guild.Rank,
		&guild.UserMember,
		&guild.UserContribution,
		&guild.Challenge.Title,
		&guild.Challenge.Description,
		&guild.Challenge.Progress,
		&guild.Challenge.Target,
	); err != nil {
		return domain.Guild{}, fmt.Errorf("query suggested guild: %w", err)
	}

	feed, err := r.guildFeed(ctx, guild.ID)
	if err != nil {
		return domain.Guild{}, err
	}

	guild.Feed = feed

	return guild, nil
}

func (r *Repository) JoinGuild(
	ctx context.Context,
	userID string,
	guildID string,
) error {
	if _, err := r.database.Exec(ctx, `
		INSERT INTO guild_memberships (guild_id, user_id)
		SELECT guild.id, user_row.id
		FROM guilds guild
		JOIN users user_row ON user_row.id = $1
		WHERE guild.id = $2 AND guild.territory_id = user_row.home_city_id
		ON CONFLICT DO NOTHING
	`, userID, guildID); err != nil {
		return fmt.Errorf("insert guild membership: %w", err)
	}

	return nil
}

func (r *Repository) LeaveGuild(ctx context.Context, userID string) error {
	if _, err := r.database.Exec(ctx, `
		UPDATE guild_memberships
		SET left_at = now()
		WHERE user_id = $1 AND left_at IS NULL
	`, userID); err != nil {
		return fmt.Errorf("leave guild membership: %w", err)
	}

	return nil
}

func (r *Repository) TravelCohort(
	ctx context.Context,
	userID string,
	territoryID string,
) (domain.TravelCohort, error) {
	var cohort domain.TravelCohort

	var windowStart, windowEnd time.Time

	err := r.database.QueryRow(ctx, `
		SELECT
			cohort.demo_aggregate_count + COUNT(membership.user_id) FILTER (
				WHERE membership.left_at IS NULL AND membership.visibility = 'aggregate'
			),
			cohort.demo_guild_count,
			(setting.value #>> '{}')::int,
			cohort.window_start,
			cohort.window_end
		FROM users user_row
		JOIN travel_cohorts cohort
			ON cohort.destination_city_id = $2
			AND (cohort.origin_city_id IS NULL OR cohort.origin_city_id = user_row.home_city_id)
		JOIN app_settings setting ON setting.key = 'privacy_threshold'
		LEFT JOIN travel_cohort_memberships membership ON membership.cohort_id = cohort.id
		WHERE user_row.id = $1 AND cohort.expires_at > now()
		GROUP BY cohort.id, setting.value
		ORDER BY cohort.window_start
		LIMIT 1
	`, userID, territoryID).Scan(
		&cohort.Count,
		&cohort.FromGuild,
		&cohort.Threshold,
		&windowStart,
		&windowEnd,
	)
	if postgres.IsNotFound(err) {
		return domain.TravelCohort{
			Window:  "",
			Privacy: "k-anonymity",
			Message: "В этом направлении пока небольшая группа",
		}, nil
	}

	if err != nil {
		return domain.TravelCohort{}, fmt.Errorf("query travel cohort: %w", err)
	}

	cohort.Visible = cohort.Count >= cohort.Threshold
	cohort.Window = formatWindow(windowStart, windowEnd)
	cohort.Privacy = "k-anonymity"
	cohort.Demo = true

	if !cohort.Visible {
		cohort.Message = "В этом направлении пока небольшая группа"
		cohort.Count = 0
		cohort.FromGuild = 0
	}

	return cohort, nil
}

func (r *Repository) guildFeed(
	ctx context.Context,
	guildID string,
) ([]domain.GuildFeedItem, error) {
	rows, err := r.database.Query(ctx, `
		SELECT id, text, points, created_at
		FROM guild_feed
		WHERE guild_id = $1
		ORDER BY created_at DESC
		LIMIT 20
	`, guildID)
	if err != nil {
		return nil, fmt.Errorf("query guild feed: %w", err)
	}
	defer rows.Close()

	items := make([]domain.GuildFeedItem, 0)

	for rows.Next() {
		var item domain.GuildFeedItem

		var createdAt time.Time
		if err := rows.Scan(&item.ID, &item.Text, &item.Points, &createdAt); err != nil {
			return nil, fmt.Errorf("scan guild feed item: %w", err)
		}

		item.Ago = humanAge(time.Since(createdAt))
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate guild feed: %w", err)
	}

	return items, nil
}

func (r *Repository) leagueRules(ctx context.Context) ([]leagueRule, error) {
	var payload []byte
	if err := r.database.QueryRow(
		ctx,
		`SELECT value FROM app_settings WHERE key = 'league_rules'`,
	).Scan(&payload); err != nil {
		return nil, fmt.Errorf("query league rules: %w", err)
	}

	var rules []leagueRule
	if err := json.Unmarshal(payload, &rules); err != nil {
		return nil, fmt.Errorf("decode league rules: %w", err)
	}

	if len(rules) == 0 {
		return nil, community_errors.ErrLeagueRulesConfiguration
	}

	return rules, nil
}

func leagueForScore(score int, rules []leagueRule) (string, int, int, error) {
	ordered := append([]leagueRule(nil), rules...)
	sort.SliceStable(ordered, func(left int, right int) bool {
		return ordered[left].MinimumScore > ordered[right].MinimumScore
	})

	for _, rule := range ordered {
		if score >= rule.MinimumScore && rule.Name != "" {
			return rule.Name, rule.NextScore, rule.Percentile, nil
		}
	}

	return "", 0, 0, community_errors.ErrLeagueRulesConfiguration
}

func formatWindow(start time.Time, end time.Time) string {
	if start.IsZero() || end.IsZero() {
		return ""
	}

	return fmt.Sprintf(
		"%s — %s",
		start.Local().Format("02.01"),
		end.Local().Format("02.01"),
	)
}

func humanAge(age time.Duration) string {
	if age < time.Hour {
		return fmt.Sprintf("%d мин", int(age.Minutes()))
	}

	return fmt.Sprintf("%d ч", int(age.Hours()))
}
