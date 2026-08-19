package tutumcp

import "encoding/json"

type Price struct {
	Amount   json.Number `json:"amount"`
	Currency string      `json:"currency,omitempty"`

	PriceBasis string `json:"price_basis,omitempty"`
}

func (p *Price) Float() (float64, bool) {
	if p == nil {
		return 0, false
	}

	value, err := p.Amount.Float64()
	if err != nil {
		return 0, false
	}

	return value, true
}

func (p *Price) String() string {
	if p == nil {
		return ""
	}

	if p.Currency == "" {
		return p.Amount.String()
	}

	return p.Amount.String() + " " + p.Currency
}

type GeoPoint struct {
	GeoID       string `json:"geo_id,omitempty"`
	Name        string `json:"name,omitempty"`
	Region      string `json:"region,omitempty"`
	Country     string `json:"country,omitempty"`
	IATA        string `json:"iata,omitempty"`
	GeoType     string `json:"geo_type,omitempty"`
	HotelsCount int    `json:"hotels_count,omitempty"`
}

type Carrier struct {
	Name        string      `json:"name"`
	OffersCount int         `json:"offers_count,omitempty"`
	PriceFrom   json.Number `json:"price_from,omitempty"`
}

type Meta struct {
	Transport         string          `json:"transport,omitempty"`
	From              *GeoPoint       `json:"from,omitempty"`
	To                *GeoPoint       `json:"to,omitempty"`
	ResolvedGeo       *GeoPoint       `json:"resolved_geo,omitempty"`
	Sort              string          `json:"sort,omitempty"`
	Page              int             `json:"page,omitempty"`
	PageSize          int             `json:"page_size,omitempty"`
	TotalReturned     int             `json:"total_returned,omitempty"`
	TotalMatched      int             `json:"total_matched,omitempty"`
	TotalMatchedExact bool            `json:"total_matched_exact,omitempty"`
	HasMore           bool            `json:"has_more,omitempty"`
	SearchID          string          `json:"search_id,omitempty"`
	CarriersAvailable []Carrier       `json:"carriers_available,omitempty"`
	ModesSummary      json.RawMessage `json:"modes_summary,omitempty"`
	Unavailable       json.RawMessage `json:"unavailable,omitempty"`
	InterchangeRoutes json.RawMessage `json:"interchange_routes,omitempty"`
}

type Segment struct {
	From        string `json:"from,omitempty"`
	To          string `json:"to,omitempty"`
	DepartureAt string `json:"departure_at,omitempty"`
	ArrivalAt   string `json:"arrival_at,omitempty"`
	DurationMin int    `json:"duration_min,omitempty"`
	Carrier     string `json:"carrier,omitempty"`
	VoyageNo    string `json:"voyage_no,omitempty"`
}

type Leg struct {
	Label       string    `json:"label,omitempty"`
	From        string    `json:"from,omitempty"`
	To          string    `json:"to,omitempty"`
	DepartureAt string    `json:"departure_at,omitempty"`
	ArrivalAt   string    `json:"arrival_at,omitempty"`
	DurationMin int       `json:"duration_min,omitempty"`
	Segments    []Segment `json:"segments,omitempty"`
}

type BestOffer struct {
	OfferPackHash string `json:"offerpack_hash,omitempty"`
	RoomName      string `json:"room_name,omitempty"`
	Price         *Price `json:"price,omitempty"`
}

type Offer struct {
	OfferID          string          `json:"offer_id,omitempty"`
	Transport        string          `json:"transport,omitempty"`
	Price            *Price          `json:"price,omitempty"`
	DurationMin      int             `json:"duration_min,omitempty"`
	Carriers         []string        `json:"carriers,omitempty"`
	SegmentsCount    int             `json:"segments_count,omitempty"`
	DepartureAt      string          `json:"departure_at,omitempty"`
	ArrivalAt        string          `json:"arrival_at,omitempty"`
	Legs             []Leg           `json:"legs,omitempty"`
	SearchResultsURL string          `json:"search_results_url,omitempty"`
	CheckoutURL      string          `json:"checkout_url,omitempty"`
	DetailsRef       json.RawMessage `json:"details_ref,omitempty"`
	CheckoutRef      json.RawMessage `json:"checkout_ref,omitempty"`

	HotelID     string     `json:"hotel_id,omitempty"`
	HotelGeoID  string     `json:"hotel_geo_id,omitempty"`
	TutuOfferID string     `json:"tutu_offer_id,omitempty"`
	Name        string     `json:"name,omitempty"`
	Address     string     `json:"address,omitempty"`
	Stars       int        `json:"stars,omitempty"`
	Rating      float64    `json:"rating,omitempty"`
	ReviewCount int        `json:"review_count,omitempty"`
	Photos      []string   `json:"photos,omitempty"`
	BestOffer   *BestOffer `json:"best_offer,omitempty"`

	Raw json.RawMessage `json:"-"`
}

func (o *Offer) UnmarshalJSON(data []byte) error {
	type plain Offer

	var value plain
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}

	*o = Offer(value)

	o.Raw = append(json.RawMessage(nil), data...)

	return nil
}

func (o *Offer) Decode(v any) error {
	if o == nil || len(o.Raw) == 0 {
		return ErrNoPayload
	}

	return json.Unmarshal(o.Raw, v)
}

func (o *Offer) Title() string {
	if o == nil {
		return ""
	}

	if o.Name != "" {
		return o.Name
	}

	if len(o.Carriers) > 0 {
		return o.Carriers[0]
	}

	return o.Transport
}

type Stay struct {
	CheckIn  string `json:"check_in,omitempty"`
	CheckOut string `json:"check_out,omitempty"`
	Nights   int    `json:"nights,omitempty"`
}

type SearchResult struct {
	Offers   []Offer `json:"offers,omitempty"`
	Variants []Offer `json:"variants,omitempty"`
	Hotels   []Offer `json:"hotels,omitempty"`
	Stay     *Stay   `json:"stay,omitempty"`
	Meta     Meta    `json:"meta"`

	Raw json.RawMessage `json:"-"`
}

func (r *SearchResult) Items() []Offer {
	if r == nil {
		return nil
	}

	switch {
	case len(r.Offers) > 0:
		return r.Offers
	case len(r.Variants) > 0:
		return r.Variants
	default:
		return r.Hotels
	}
}

func (r *SearchResult) HasMore() bool {
	return r != nil && r.Meta.HasMore
}

func (r *SearchResult) Cheapest() (Offer, bool) {
	var (
		best  Offer
		found bool
		min   float64
	)

	for _, offer := range r.Items() {
		price, ok := offer.Price.Float()
		if !ok {
			continue
		}

		if !found || price < min {
			best, min, found = offer, price, true
		}
	}

	return best, found
}

func (r *SearchResult) Fastest() (Offer, bool) {
	var (
		best  Offer
		found bool
	)

	for _, offer := range r.Items() {
		if offer.DurationMin <= 0 {
			continue
		}

		if !found || offer.DurationMin < best.DurationMin {
			best, found = offer, true
		}
	}

	return best, found
}

func (r *SearchResult) Decode(v any) error {
	if r == nil || len(r.Raw) == 0 {
		return ErrNoPayload
	}

	return json.Unmarshal(r.Raw, v)
}

type OfferDetails struct {
	Raw json.RawMessage
}

func (d *OfferDetails) Decode(v any) error {
	if d == nil || len(d.Raw) == 0 {
		return ErrNoPayload
	}

	return json.Unmarshal(d.Raw, v)
}

type RailSeatmap struct {
	Cars    []RailCar       `json:"cars,omitempty"`
	Summary string          `json:"summary,omitempty"`
	Raw     json.RawMessage `json:"-"`
}

type RailCar struct {
	Number         string     `json:"number,omitempty"`
	Type           string     `json:"type,omitempty"`
	Category       string     `json:"category,omitempty"`
	AvailableSeats int        `json:"available_seats,omitempty"`
	Price          *Price     `json:"price,omitempty"`
	Seats          []RailSeat `json:"seats,omitempty"`
}

type RailSeat struct {
	Number string `json:"number,omitempty"`
	Type   string `json:"type,omitempty"`
	Price  *Price `json:"price,omitempty"`
	Female bool   `json:"female,omitempty"`
}

func (m *RailSeatmap) Decode(v any) error {
	if m == nil || len(m.Raw) == 0 {
		return ErrNoPayload
	}

	return json.Unmarshal(m.Raw, v)
}

type CheckoutLink struct {
	CheckoutURL string          `json:"checkout_url,omitempty"`
	Kind        string          `json:"kind,omitempty"`
	Raw         json.RawMessage `json:"-"`
}

func (l *CheckoutLink) IsSearchRedirect() bool {
	return l != nil && l.Kind == KindSearchRedirect
}
