package tutumcp

import (
	"encoding/json"
	"fmt"
	"time"
)

const DateLayout = "2006-01-02"

const (
	maxPage     = 10
	maxPageSize = 30
)

func Date(t time.Time) string { return t.Format(DateLayout) }

type SearchParams struct {
	Origin string `json:"origin"`

	Destination string `json:"destination"`

	DepartureDate string `json:"departure_date"`

	Page int `json:"page,omitempty"`

	PageSize int `json:"page_size,omitempty"`

	PriceMax float64 `json:"price_max,omitempty"`

	DirectOnly bool `json:"direct_only,omitempty"`

	Carriers []string `json:"carriers,omitempty"`

	View View `json:"view,omitempty"`
}

func (p SearchParams) Validate() error {
	if p.Origin == "" {
		return &ValidationError{Field: "origin", Reason: "не заполнено"}
	}

	if p.Destination == "" {
		return &ValidationError{Field: "destination", Reason: "не заполнено"}
	}

	if err := validateDate("departure_date", p.DepartureDate); err != nil {
		return err
	}

	if err := validateRange("page", p.Page, maxPage); err != nil {
		return err
	}

	return validateRange("page_size", p.PageSize, maxPageSize)
}

type AviaParams struct {
	SearchParams

	Sort         Sort         `json:"sort,omitempty"`
	ReturnDate   string       `json:"return_date,omitempty"`
	Adults       int          `json:"adults,omitempty"`
	Children     int          `json:"children,omitempty"`
	Infants      int          `json:"infants,omitempty"`
	ServiceClass ServiceClass `json:"service_class,omitempty"`
}

func (p AviaParams) Validate() error {
	if err := p.SearchParams.Validate(); err != nil {
		return err
	}

	if p.ReturnDate != "" {
		if err := validateDate("return_date", p.ReturnDate); err != nil {
			return err
		}

		if p.ReturnDate < p.DepartureDate {
			return &ValidationError{Field: "return_date", Reason: "раньше даты отправления"}
		}
	}

	return nil
}

type RailParams struct {
	SearchParams
	Sort           Sort           `json:"sort,omitempty"`
	Passengers     int            `json:"passengers,omitempty"`
	SeatCategories []SeatCategory `json:"seat_categories,omitempty"`
}

func (p RailParams) Validate() error { return p.SearchParams.Validate() }

type BusParams struct {
	SearchParams
	Sort     Sort `json:"sort,omitempty"`
	Adults   int  `json:"adults,omitempty"`
	Children int  `json:"children,omitempty"`
}

func (p BusParams) Validate() error { return p.SearchParams.Validate() }

type EtrainParams struct {
	SearchParams
	Sort Sort `json:"sort,omitempty"`
}

func (p EtrainParams) Validate() error { return p.SearchParams.Validate() }

type MultitransportParams struct {
	SearchParams
	Adults      int             `json:"adults,omitempty"`
	Modes       []TransportMode `json:"modes,omitempty"`
	OptimizeFor OptimizeFor     `json:"optimize_for,omitempty"`
}

func (p MultitransportParams) Validate() error { return p.SearchParams.Validate() }

type HotelParams struct {
	CityName          string  `json:"city_name,omitempty"`
	GeoID             string  `json:"geo_id,omitempty"`
	CheckIn           string  `json:"check_in"`
	CheckOut          string  `json:"check_out"`
	Adults            int     `json:"adults,omitempty"`
	ChildrenAges      []int   `json:"children_ages,omitempty"`
	Stars             []int   `json:"stars,omitempty"`
	MinRating         float64 `json:"min_rating,omitempty"`
	BreakfastIncluded *bool   `json:"breakfast_included,omitempty"`
	FreeCancellation  *bool   `json:"free_cancellation,omitempty"`
	PriceMax          float64 `json:"price_max,omitempty"`
	Page              int     `json:"page,omitempty"`
	PageSize          int     `json:"page_size,omitempty"`
	View              View    `json:"view,omitempty"`
}

func (p HotelParams) Validate() error {
	if p.CityName == "" && p.GeoID == "" {
		return &ValidationError{Field: "city_name", Reason: "не заполнено"}
	}

	if err := validateDate("check_in", p.CheckIn); err != nil {
		return err
	}

	if err := validateDate("check_out", p.CheckOut); err != nil {
		return err
	}

	if p.CheckOut <= p.CheckIn {
		return &ValidationError{Field: "check_out", Reason: "должна быть позже даты заезда"}
	}

	if err := validateRange("page", p.Page, maxPage); err != nil {
		return err
	}

	return validateRange("page_size", p.PageSize, maxPageSize)
}

type DetailsParams struct {
	ProductType ProductType     `json:"product_type,omitempty"`
	DetailsRef  json.RawMessage `json:"details_ref,omitempty"`
	HotelID     string          `json:"hotel_id,omitempty"`
	HotelGeoID  string          `json:"hotel_geo_id,omitempty"`
	OfferID     string          `json:"offer_id,omitempty"`
	View        string          `json:"view,omitempty"`
}

func (p DetailsParams) Validate() error {
	if len(p.DetailsRef) == 0 && p.HotelID == "" && p.HotelGeoID == "" && p.OfferID == "" {
		return &ValidationError{Field: "details_ref", Reason: "нужен details_ref или идентификатор отеля"}
	}

	return nil
}

type SeatmapParams struct {
	DetailsRef    json.RawMessage `json:"details_ref"`
	Task          SeatmapTask     `json:"task,omitempty"`
	SeatsTogether int             `json:"seats_together,omitempty"`
	CarNumber     string          `json:"car_number,omitempty"`
	View          View            `json:"view,omitempty"`
}

func (p SeatmapParams) Validate() error {
	if len(p.DetailsRef) == 0 {
		return &ValidationError{Field: "details_ref", Reason: "не заполнено"}
	}

	return nil
}

type CheckoutParams struct {
	ProductType ProductType     `json:"-"`
	CheckoutRef json.RawMessage `json:"-"`

	CarNumber   string   `json:"-"`
	SeatNumbers []string `json:"-"`

	OfferPackHash string `json:"-"`

	Extra map[string]any `json:"-"`
}

func (p CheckoutParams) Validate() error {
	if p.ProductType == "" {
		return &ValidationError{Field: "product_type", Reason: "не заполнено"}
	}

	if len(p.CheckoutRef) == 0 && len(p.Extra) == 0 {
		return &ValidationError{Field: "checkout_ref", Reason: "не заполнено"}
	}

	return nil
}

func (p CheckoutParams) arguments() (map[string]any, error) {
	args := map[string]any{}
	if len(p.CheckoutRef) > 0 {
		if err := json.Unmarshal(p.CheckoutRef, &args); err != nil {
			return nil, fmt.Errorf("tutumcp: разбор checkout_ref: %w", err)
		}
	}

	args["product_type"] = string(p.ProductType)
	if p.CarNumber != "" {
		args["car_number"] = p.CarNumber
	}

	if len(p.SeatNumbers) > 0 {
		args["seat_numbers"] = p.SeatNumbers
	}

	if p.OfferPackHash != "" {
		args["offer_pack_hash"] = p.OfferPackHash
	}

	for key, value := range p.Extra {
		args[key] = value
	}

	return args, nil
}

type validator interface {
	Validate() error
}

func validate(params validator) error {
	if params == nil {
		return nil
	}

	return params.Validate()
}

func validateDate(field, value string) error {
	if value == "" {
		return &ValidationError{Field: field, Reason: "не заполнено"}
	}

	if _, err := time.Parse(DateLayout, value); err != nil {
		return &ValidationError{Field: field, Reason: "ожидается формат YYYY-MM-DD"}
	}

	return nil
}

func validateRange(field string, value, max int) error {
	if value == 0 {
		return nil
	}

	if value < 1 || value > max {
		return &ValidationError{Field: field, Reason: fmt.Sprintf("допустимы значения от 1 до %d", max)}
	}

	return nil
}
