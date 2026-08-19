package postgres

import (
	"bytes"
	"context"
	"embed"
	"encoding/csv"
	"fmt"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	cityFixtureFields     = 5
	badgeVocabularyFields = 5
)

//go:embed fixtures/*
var fixtureFiles embed.FS //nolint:gochecknoglobals // go:embed requires a package variable.

func SeedFixtures(ctx context.Context, pool *pgxpool.Pool) error {
	if err := importBadgeVocabulary(ctx, pool); err != nil {
		return err
	}

	if err := importCities(ctx, pool); err != nil {
		return err
	}

	contentScript, err := fixtureFiles.ReadFile("fixtures/content_seed.sql")
	if err != nil {
		return fmt.Errorf("read content fixture: %w", err)
	}

	if _, err := pool.Exec(ctx, string(contentScript)); err != nil {
		return fmt.Errorf("seed database content: %w", err)
	}

	return nil
}

func readFixtureRecords(name string, fields int) ([][]string, error) {
	fixture, err := fixtureFiles.ReadFile("fixtures/" + name)
	if err != nil {
		return nil, fmt.Errorf("read fixture %s: %w", name, err)
	}

	reader := csv.NewReader(bytes.NewReader(fixture))
	reader.Comma = '\t'
	reader.Comment = '#'
	reader.FieldsPerRecord = fields

	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("parse fixture %s: %w", name, err)
	}

	return records, nil
}

func splitBadges(value string) []string {
	parts := strings.Split(value, ",")
	badges := make([]string, 0, len(parts))

	for _, part := range parts {
		badge := strings.TrimSpace(part)
		if badge == "" {
			continue
		}

		badges = append(badges, badge)
	}

	return badges
}

func importBadgeVocabulary(ctx context.Context, pool *pgxpool.Pool) error {
	records, err := readFixtureRecords("badge_vocabulary.tsv", badgeVocabularyFields)
	if err != nil {
		return err
	}

	labels := make([]string, 0, len(records))

	for _, record := range records {
		label := strings.TrimSpace(record[0])
		if label == "" {
			continue
		}

		order, parseErr := strconv.Atoi(strings.TrimSpace(record[4]))
		if parseErr != nil {
			return fmt.Errorf("parse sort order for badge %s: %w", label, parseErr)
		}

		if _, err := pool.Exec(
			ctx,
			upsertBadgeSQL,
			label,
			strings.TrimSpace(record[1]),
			strings.TrimSpace(record[2]),
			splitBadges(record[3]),
			order,
		); err != nil {
			return fmt.Errorf("import badge %s: %w", label, err)
		}

		labels = append(labels, label)
	}

	if _, err := pool.Exec(ctx, deactivateBadgesSQL, labels); err != nil {
		return fmt.Errorf("deactivate stale badges: %w", err)
	}

	return nil
}

func importCities(ctx context.Context, pool *pgxpool.Pool) error {
	records, err := readFixtureRecords("ru_cities.tsv", cityFixtureFields)
	if err != nil {
		return err
	}

	transaction, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin city fixture import: %w", err)
	}

	defer func() {
		_ = transaction.Rollback(ctx)
	}()

	for _, record := range records {
		if err := insertCityFixture(ctx, transaction, record); err != nil {
			return err
		}
	}

	if err := transaction.Commit(ctx); err != nil {
		return fmt.Errorf("commit city fixture import: %w", err)
	}

	return nil
}

func insertCityFixture(
	ctx context.Context,
	executor pgx.Tx,
	record []string,
) error {
	latitude, err := strconv.ParseFloat(record[2], 64)
	if err != nil {
		return fmt.Errorf("parse latitude for GeoNames %s: %w", record[0], err)
	}

	longitude, err := strconv.ParseFloat(record[3], 64)
	if err != nil {
		return fmt.Errorf("parse longitude for GeoNames %s: %w", record[0], err)
	}

	badges := splitBadges(record[4])

	if _, err := executor.Exec(
		ctx,
		insertCityFixtureSQL,
		record[0],
		record[1],
		latitude,
		longitude,
		badges,
	); err != nil {
		return fmt.Errorf("import GeoNames city %s: %w", record[0], err)
	}

	if len(badges) == 0 {
		return nil
	}

	if _, err := executor.Exec(
		ctx,
		applyCityBadgesSQL,
		record[1],
		latitude,
		longitude,
		badges,
		record[0],
	); err != nil {
		return fmt.Errorf("apply badges for city %s: %w", record[1], err)
	}

	return nil
}

const insertCityFixtureSQL = `
	INSERT INTO territories (
		id,
		name,
		slug,
		region,
		centroid,
		tags,
		rarity,
		reward,
		promo_percent,
		description,
		image_tone,
		source_code,
		external_id,
		badges
	)
	SELECT
		md5('geonames:' || $1)::uuid,
		$2,
		'geonames-' || $1,
		'Россия',
		ST_SetSRID(ST_MakePoint($4::double precision, $3::double precision), 4326)::geography,
		CASE
			WHEN $3::double precision > 62 THEN ARRAY['north','nature','unusual']
			WHEN $4::double precision > 90 THEN ARRAY['nature','history','unusual']
			WHEN $4::double precision BETWEEN 40 AND 65 THEN ARRAY['history','food','architecture']
			ELSE ARRAY['architecture','events','calm']
		END,
		CASE WHEN $3::double precision > 62 OR $4::double precision > 100 THEN 4 ELSE 2 END,
		CASE WHEN $3::double precision > 62 OR $4::double precision > 100 THEN 160 ELSE 120 END,
		CASE
			WHEN $3::double precision > 62 OR $4::double precision > 100 THEN 10
			WHEN abs(hashtext('geonames-' || $1)) % 100 >= 35 THEN 0
			ELSE 5
		END,
		$2 || ' — одна из точек большого каталога российских путешествий.',
		(ARRAY['lilac','green','orange','blue'])[(abs(hashtext($1)) % 4) + 1],
		'geonames',
		$1,
		$5::text[]
	WHERE NOT EXISTS (
		SELECT 1
		FROM territories existing
		WHERE ST_DWithin(
			existing.centroid,
			ST_SetSRID(ST_MakePoint($4::double precision, $3::double precision), 4326)::geography,
			5000
		)
	)
	ON CONFLICT (source_code, external_id) WHERE external_id IS NOT NULL DO UPDATE SET
		name = EXCLUDED.name,
		centroid = EXCLUDED.centroid,
		badges = EXCLUDED.badges,
		active = TRUE`

const applyCityBadgesSQL = `
	UPDATE territories
	SET badges = $4::text[],
		name = CASE
			WHEN source_code = 'geonames' AND external_id = $5 THEN $1
			ELSE name
		END
	WHERE (source_code = 'geonames' AND external_id = $5)
	   OR (
			lower(name) = lower($1)
			AND ST_DWithin(
				centroid,
				ST_SetSRID(ST_MakePoint($3::double precision, $2::double precision), 4326)::geography,
				60000
			)
		)`

const upsertBadgeSQL = `
	INSERT INTO badge_catalog (label, group_label, icon, related, sort_order)
	VALUES ($1, $2, $3, $4::text[], $5)
	ON CONFLICT (label) DO UPDATE SET
		group_label = EXCLUDED.group_label,
		icon = EXCLUDED.icon,
		related = EXCLUDED.related,
		sort_order = EXCLUDED.sort_order,
		active = TRUE`

const deactivateBadgesSQL = `
	UPDATE badge_catalog
	SET active = FALSE
	WHERE NOT (label = ANY($1::text[]))`
