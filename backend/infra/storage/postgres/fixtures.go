package postgres

import (
	"bytes"
	"context"
	"embed"
	"encoding/csv"
	"fmt"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed fixtures/*
var fixtureFiles embed.FS //nolint:gochecknoglobals // go:embed requires a package variable.

func SeedFixtures(ctx context.Context, pool *pgxpool.Pool) error {
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

func importCities(ctx context.Context, pool *pgxpool.Pool) error {
	fixture, err := fixtureFiles.ReadFile("fixtures/ru_cities.tsv")
	if err != nil {
		return fmt.Errorf("read city fixture: %w", err)
	}

	reader := csv.NewReader(bytes.NewReader(fixture))
	reader.Comma = '\t'
	reader.Comment = '#'
	reader.FieldsPerRecord = 4

	records, err := reader.ReadAll()
	if err != nil {
		return fmt.Errorf("parse city fixture: %w", err)
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

	if _, err := executor.Exec(
		ctx,
		insertCityFixtureSQL,
		record[0],
		record[1],
		latitude,
		longitude,
	); err != nil {
		return fmt.Errorf("import GeoNames city %s: %w", record[0], err)
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
		external_id
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
		$1
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
		active = TRUE`
