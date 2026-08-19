package tutu_transport

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tutu-hack/openworld/infra/storage/postgres"
	"github.com/tutu-hack/openworld/infra/tutumcp"
	mcp_errors "github.com/tutu-hack/openworld/internal/errors/mcp"
	"github.com/tutu-hack/openworld/internal/models/domain"
	"golang.org/x/sync/errgroup"
)

const searchConcurrency = 4

type Provider struct {
	database *pgxpool.Pool
	client   *tutumcp.Client
	logger   *slog.Logger
}

func NewProvider(database *pgxpool.Pool, client *tutumcp.Client, logger *slog.Logger) *Provider {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}

	return &Provider{database: database, client: client, logger: logger}
}

func (p *Provider) Search(
	ctx context.Context,
	recommendation domain.RecommendationRequest,
	candidates []domain.Territory,
	settings domain.RecommendationSettings,
) (domain.TransportSearchResult, error) {
	originName, err := p.cityName(ctx, recommendation.OriginCityID)
	if err != nil {
		return domain.TransportSearchResult{}, err
	}

	searchLimit := min(settings.CandidateSearchLimit, len(candidates))
	selected := candidates[:searchLimit]
	outcomes := make([]candidateOutcome, len(selected))

	group, groupContext := errgroup.WithContext(ctx)
	group.SetLimit(searchConcurrency)

	for index, candidate := range selected {
		index, candidate := index, candidate

		group.Go(func() error {
			outcomes[index] = p.searchCandidate(
				groupContext,
				recommendation,
				candidate,
				settings,
				originName,
			)

			return outcomes[index].fatal
		})
	}

	if err := group.Wait(); err != nil {
		return domain.TransportSearchResult{}, err
	}

	offers := make([]domain.TransportOffer, 0, searchLimit)
	issues := make([]domain.TransportSearchIssue, 0)

	for _, outcome := range outcomes {
		if outcome.issue != nil {
			issues = append(issues, *outcome.issue)

			continue
		}

		if outcome.offer != nil {
			offers = append(offers, *outcome.offer)
		}
	}

	if len(offers) == 0 {
		return domain.TransportSearchResult{}, mcp_errors.ErrNoOffers
	}

	sort.SliceStable(offers, func(left int, right int) bool {
		return offers[left].Price < offers[right].Price
	})

	return domain.TransportSearchResult{Offers: offers, Issues: issues}, nil
}

type candidateOutcome struct {
	offer *domain.TransportOffer
	issue *domain.TransportSearchIssue
	fatal error
}

func (p *Provider) searchCandidate(
	ctx context.Context,
	recommendation domain.RecommendationRequest,
	candidate domain.Territory,
	settings domain.RecommendationSettings,
	originName string,
) candidateOutcome {
	result, searchErr := p.client.SearchMultitransport(
		ctx,
		tutumcp.MultitransportParams{
			SearchParams: tutumcp.SearchParams{
				Origin:        originName,
				Destination:   candidate.Name,
				DepartureDate: recommendation.DateFrom,
				PageSize:      settings.OfferPageSize,
				PriceMax:      float64(recommendation.Budget),
				DirectOnly:    recommendation.DirectOnly,
				View:          tutumcp.ViewCompact,
			},
			Adults:      recommendation.Adults + recommendation.Children,
			Modes:       transportModes(recommendation.TransportModes),
			OptimizeFor: optimizeFor(recommendation.MaxTravelMinutes),
		},
	)
	if searchErr != nil {
		p.logger.Warn(
			"tutu mcp multitransport search failed",
			"city_id", candidate.ID,
			"city", candidate.Name,
			"error", searchErr,
		)

		return candidateOutcome{
			issue: &domain.TransportSearchIssue{CityID: candidate.ID, Code: "MCP_SEARCH_FAILED"},
		}
	}

	offer, found := cheapestValidOffer(result.Items(), recommendation)
	if !found {
		p.logger.Info(
			"tutu mcp offers rejected by trip constraints",
			"city_id", candidate.ID,
			"city", candidate.Name,
			"offers", describeOffers(result.Items()),
			"reason", "no bookable offer within budget, currency and transport filters",
			"requested_modes", recommendation.TransportModes,
			"budget", recommendation.Budget,
			"currency", recommendation.Currency,
			"max_travel_minutes", recommendation.MaxTravelMinutes,
		)

		return candidateOutcome{
			issue: &domain.TransportSearchIssue{CityID: candidate.ID, Code: "MCP_NO_VALID_OFFER"},
		}
	}

	price, validPrice := offer.Price.Float()
	if !validPrice || price < 0 || price > float64(math.MaxInt) {
		return candidateOutcome{
			issue: &domain.TransportSearchIssue{CityID: candidate.ID, Code: "MCP_INVALID_PRICE"},
		}
	}

	snapshot, marshalErr := json.Marshal(offer)
	if marshalErr != nil {
		return candidateOutcome{fatal: fmt.Errorf("encode Tutu MCP offer: %w", marshalErr)}
	}

	return candidateOutcome{offer: &domain.TransportOffer{
		CityID:          candidate.ID,
		CityName:        candidate.Name,
		Transport:       offer.Transport,
		Price:           int(math.Round(price)),
		Currency:        strings.ToUpper(offer.Price.Currency),
		DurationMinutes: offer.DurationMin,
		DepartureAt:     offer.DepartureAt,
		ArrivalAt:       offer.ArrivalAt,
		CheckoutRef:     append(json.RawMessage(nil), offer.CheckoutRef...),
		Snapshot:        snapshot,
		Source:          "tutu_mcp",
	}}
}

func describeOffers(offers []tutumcp.Offer) []string {
	described := make([]string, 0, len(offers))

	for _, offer := range offers {
		price, _ := offer.Price.Float()
		described = append(described, fmt.Sprintf(
			"%s %.0f %s %dmin",
			offer.Transport, price, offer.Price.Currency, offer.DurationMin,
		))
	}

	return described
}

func optimizeFor(maxTravelMinutes int) tutumcp.OptimizeFor {
	if maxTravelMinutes > 0 {
		return tutumcp.OptimizeTime
	}

	return tutumcp.OptimizePrice
}

func cheapestValidOffer(
	offers []tutumcp.Offer,
	recommendation domain.RecommendationRequest,
) (tutumcp.Offer, bool) {
	suitable := make([]tutumcp.Offer, 0, len(offers))

	for _, offer := range offers {
		if bookableOffer(offer, recommendation) {
			suitable = append(suitable, offer)
		}
	}

	if len(suitable) == 0 {
		return tutumcp.Offer{}, false
	}

	sort.SliceStable(suitable, func(left int, right int) bool {
		leftPrice, _ := suitable[left].Price.Float()
		rightPrice, _ := suitable[right].Price.Float()

		return leftPrice < rightPrice
	})

	if recommendation.MaxTravelMinutes <= 0 {
		return suitable[0], true
	}

	for _, offer := range suitable {
		if offer.DurationMin <= recommendation.MaxTravelMinutes {
			return offer, true
		}
	}

	fastest := suitable[0]
	for _, offer := range suitable[1:] {
		if offer.DurationMin < fastest.DurationMin {
			fastest = offer
		}
	}

	return fastest, true
}

func bookableOffer(offer tutumcp.Offer, recommendation domain.RecommendationRequest) bool {
	price, priceOK := offer.Price.Float()
	departure, departureOK := parseOfferTime(offer.DepartureAt)
	arrival, arrivalOK := parseOfferTime(offer.ArrivalAt)

	return priceOK && price >= 0 && price <= float64(recommendation.Budget) &&
		strings.EqualFold(offer.Price.Currency, recommendation.Currency) &&
		offer.DurationMin > 0 &&
		len(offer.CheckoutRef) > 0 &&
		departureOK && arrivalOK && !arrival.Before(departure) &&
		allowedTransportMode(offer.Transport, recommendation.TransportModes)
}

func parseOfferTime(value string) (time.Time, bool) {
	parsed, err := time.Parse(time.RFC3339, value)

	return parsed, err == nil
}

func allowedTransportMode(mode string, allowed []string) bool {
	for _, item := range allowed {
		if item == mode {
			return true
		}
	}

	return false
}

func (p *Provider) cityName(ctx context.Context, cityID string) (string, error) {
	var cityName string

	err := p.database.QueryRow(ctx, `
		SELECT name FROM territories WHERE id = $1 AND active
	`, cityID).Scan(&cityName)
	if postgres.IsNotFound(err) {
		return "", mcp_errors.ErrOriginNotFound
	}

	if err != nil {
		return "", fmt.Errorf("query origin city: %w", err)
	}

	if strings.TrimSpace(cityName) == "" {
		return "", mcp_errors.ErrOriginNotFound
	}

	return cityName, nil
}

func transportModes(values []string) []tutumcp.TransportMode {
	modes := make([]tutumcp.TransportMode, 0, len(values))

	for _, value := range values {
		switch value {
		case string(tutumcp.ModeAvia):
			modes = append(modes, tutumcp.ModeAvia)
		case string(tutumcp.ModeRailway):
			modes = append(modes, tutumcp.ModeRailway)
		case string(tutumcp.ModeBus):
			modes = append(modes, tutumcp.ModeBus)
		case string(tutumcp.ModeEtrain):
			modes = append(modes, tutumcp.ModeEtrain)
		}
	}

	return modes
}
