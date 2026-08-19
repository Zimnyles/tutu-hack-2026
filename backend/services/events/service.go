package events

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/tutu-hack/openworld/internal/models/domain"
)

type Filters struct {
	DateFrom     time.Time
	DateTo       time.Time
	Category     string
	PriceMax     int
	AgeRating    string
	Availability string
}

const (
	stateWriteTimeout       = 5 * time.Second
	defaultCacheTTL         = 24 * time.Hour
	defaultFailureBackoff   = time.Hour
	defaultRunTimeout       = 5 * time.Minute
	defaultCityLimit        = 10
	defaultPopularLimit     = 12
	defaultPopularCityPool  = 120
	defaultWindowDays       = 60
	defaultDiscoverySlots   = 3
	minimumDiscoverySlots   = 1
	minimumDiscoveryTimeout = time.Minute
)

type Config struct {
	DiscoveryEnabled bool
	CacheTTL         time.Duration
	FailureBackoff   time.Duration
	RunTimeout       time.Duration
	CityLimit        int
	PopularLimit     int
	PopularCityPool  int
	WindowDays       int
	Concurrency      int
}

type Dependencies struct {
	Repository Repository
	Enricher   EventEnricher
	Discoverer EventDiscoverer
	Logger     *slog.Logger
	Config     Config
}

type Service struct {
	repository Repository
	enricher   EventEnricher
	discoverer EventDiscoverer
	logger     *slog.Logger
	config     Config
	slots      chan struct{}
}

func New(dependencies Dependencies) *Service {
	config := withDefaults(dependencies.Config)

	return &Service{
		repository: dependencies.Repository,
		enricher:   dependencies.Enricher,
		discoverer: dependencies.Discoverer,
		logger:     discoveryLogger(dependencies.Logger),
		config:     config,
		slots:      make(chan struct{}, config.Concurrency),
	}
}

func withDefaults(config Config) Config {
	if config.CacheTTL <= 0 {
		config.CacheTTL = defaultCacheTTL
	}

	if config.FailureBackoff <= 0 {
		config.FailureBackoff = defaultFailureBackoff
	}

	if config.RunTimeout < minimumDiscoveryTimeout {
		config.RunTimeout = defaultRunTimeout
	}

	if config.CityLimit <= 0 {
		config.CityLimit = defaultCityLimit
	}

	if config.PopularLimit <= 0 {
		config.PopularLimit = defaultPopularLimit
	}

	if config.PopularCityPool <= 0 {
		config.PopularCityPool = defaultPopularCityPool
	}

	if config.WindowDays <= 0 {
		config.WindowDays = defaultWindowDays
	}

	if config.Concurrency < minimumDiscoverySlots {
		config.Concurrency = defaultDiscoverySlots
	}

	return config
}

func (s *Service) List(
	ctx context.Context,
	cityID string,
	filters Filters,
) ([]domain.Event, error) {
	events, err := s.repository.List(ctx, cityID, filters)
	if err != nil {
		return nil, fmt.Errorf("list city events: %w", err)
	}

	return events, nil
}

func (s *Service) Get(
	ctx context.Context,
	eventID string,
) (domain.Event, bool, error) {
	event, found, err := s.repository.Get(ctx, eventID)
	if err != nil {
		return domain.Event{}, false, fmt.Errorf("get event: %w", err)
	}

	return event, found, nil
}

func (s *Service) Refresh(
	ctx context.Context,
	city domain.Territory,
	dateFrom time.Time,
	dateTo time.Time,
) ([]domain.Event, error) {
	current, err := s.repository.List(ctx, city.ID, Filters{DateFrom: dateFrom, DateTo: dateTo})
	if err != nil {
		return nil, fmt.Errorf("load city events for enrichment: %w", err)
	}

	if len(current) == 0 {
		return current, nil
	}

	enriched, err := s.enricher.Enrich(ctx, city, current)
	if err != nil {
		return nil, fmt.Errorf("enrich city events: %w", err)
	}

	if err := s.repository.SaveAIEnrichment(ctx, city.ID, enriched); err != nil {
		return nil, fmt.Errorf("save city event enrichment: %w", err)
	}

	persisted, err := s.repository.List(ctx, city.ID, Filters{
		DateFrom: dateFrom,
		DateTo:   dateTo,
	})
	if err != nil {
		return nil, fmt.Errorf("reload enriched city events: %w", err)
	}

	return persisted, nil
}
