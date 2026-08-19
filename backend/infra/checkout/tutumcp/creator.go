package tutu_checkout

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/tutu-hack/openworld/infra/tutumcp"
	mcp_errors "github.com/tutu-hack/openworld/internal/errors/mcp"
	"github.com/tutu-hack/openworld/internal/models/domain"
)

type Creator struct {
	client          *tutumcp.Client
	allowedHosts    map[string]struct{}
	allowedSuffixes []string
}

func NewCreator(client *tutumcp.Client, allowedHosts []string) *Creator {
	hosts := make(map[string]struct{}, len(allowedHosts))
	suffixes := make([]string, 0, len(allowedHosts))

	for _, host := range allowedHosts {
		normalized := strings.ToLower(strings.TrimSpace(host))
		if normalized == "" {
			continue
		}

		if base := strings.TrimPrefix(strings.TrimPrefix(normalized, "*"), "."); base != normalized {
			if base != "" {
				suffixes = append(suffixes, "."+base)
				hosts[base] = struct{}{}
			}

			continue
		}

		hosts[normalized] = struct{}{}
	}

	return &Creator{client: client, allowedHosts: hosts, allowedSuffixes: suffixes}
}

func (c *Creator) Create(
	ctx context.Context,
	trip domain.Trip,
) (string, string, bool, error) {
	if len(trip.Option.OfferSnapshot) == 0 || len(trip.Option.CheckoutRef) == 0 {
		return "", "", false, mcp_errors.ErrCheckoutReferenceMissing
	}

	var offer tutumcp.Offer
	if err := json.Unmarshal(trip.Option.OfferSnapshot, &offer); err != nil {
		return "", "", false, fmt.Errorf("decode saved Tutu MCP offer: %w", err)
	}

	offer.CheckoutRef = trip.Option.CheckoutRef

	link, err := c.client.CheckoutLinkFor(ctx, productType(trip.Option.Transport), offer)
	if err != nil {
		return "", "", false, fmt.Errorf("create checkout link through Tutu MCP: %w", err)
	}

	if link.CheckoutURL == "" {
		return "", "", false, mcp_errors.ErrCheckoutReferenceMissing
	}

	if err := c.validateURL(link.CheckoutURL); err != nil {
		return "", "", false, err
	}

	return link.CheckoutURL, link.Kind, false, nil
}

func (c *Creator) validateURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("%w: %s", mcp_errors.ErrCheckoutURLNotAllowed, rawURL)
	}

	if parsed.Scheme != "https" {
		return fmt.Errorf("%w: scheme %s", mcp_errors.ErrCheckoutURLNotAllowed, parsed.Scheme)
	}

	host := strings.ToLower(parsed.Hostname())
	if c.allowedHost(host) {
		return nil
	}

	return fmt.Errorf("%w: host %s", mcp_errors.ErrCheckoutURLNotAllowed, host)
}

func (c *Creator) allowedHost(host string) bool {
	if _, allowed := c.allowedHosts[host]; allowed {
		return true
	}

	for _, suffix := range c.allowedSuffixes {
		if strings.HasSuffix(host, suffix) {
			return true
		}
	}

	return false
}

func productType(transport string) tutumcp.ProductType {
	switch transport {
	case string(tutumcp.ModeAvia):
		return tutumcp.ProductAvia
	case string(tutumcp.ModeBus):
		return tutumcp.ProductBus
	case string(tutumcp.ModeEtrain):
		return tutumcp.ProductEtrain
	default:
		return tutumcp.ProductRail
	}
}
