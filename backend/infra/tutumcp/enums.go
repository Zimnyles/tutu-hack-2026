package tutumcp

const (
	ToolSearchHotels         = "search_hotels"
	ToolSearchAvia           = "search_avia"
	ToolSearchRail           = "search_rail"
	ToolSearchBus            = "search_bus"
	ToolSearchEtrain         = "search_etrain"
	ToolSearchMultitransport = "search_multitransport"

	ToolGetOfferDetails    = "get_offer_details"
	ToolGetRailSeatmap     = "get_rail_seatmap"
	ToolCreateCheckoutLink = "create_checkout_link"
	ToolFetchResource      = "fetch_resource"
)

type View string

const (
	ViewCompact View = "compact"

	ViewFull View = "full"
)

type HotelDetailsView string

const (
	HotelViewCompact HotelDetailsView = "compact"
	HotelViewRules   HotelDetailsView = "rules"
	HotelViewReviews HotelDetailsView = "reviews"
	HotelViewFull    HotelDetailsView = "full"
)

type Sort string

const (
	SortPriceAsc     Sort = "price_asc"
	SortPriceDesc    Sort = "price_desc"
	SortDurationAsc  Sort = "duration_asc"
	SortDepartureAsc Sort = "departure_asc"
)

type TransportMode string

const (
	ModeAvia    TransportMode = "avia"
	ModeRailway TransportMode = "railway"
	ModeBus     TransportMode = "bus"
	ModeEtrain  TransportMode = "etrain"
)

type OptimizeFor string

const (
	OptimizePrice OptimizeFor = "price"
	OptimizeTime  OptimizeFor = "time"
)

type ProductType string

const (
	ProductAvia   ProductType = "avia"
	ProductRail   ProductType = "rail"
	ProductBus    ProductType = "bus"
	ProductEtrain ProductType = "etrain"
	ProductHotel  ProductType = "hotel"
)

type ServiceClass string

const (
	ClassEconomy        ServiceClass = "Y"
	ClassPremiumEconomy ServiceClass = "S"
	ClassBusiness       ServiceClass = "C"
	ClassFirst          ServiceClass = "F"
)

type SeatCategory string

const (
	SeatSedentary    SeatCategory = "SEDENTARY"
	SeatReservedSeat SeatCategory = "RESERVED_SEAT"
	SeatCompartment  SeatCategory = "COMPARTMENT"
	SeatLux          SeatCategory = "LUX"
	SeatSoft         SeatCategory = "SOFT"
	SeatShared       SeatCategory = "SHARED"
)

type SeatmapTask string

const (
	SeatmapTogether  SeatmapTask = "together"
	SeatmapFarFromWC SeatmapTask = "far_from_wc"
	SeatmapFemale    SeatmapTask = "female"
	SeatmapSummary   SeatmapTask = "summary"
)

type Domain string

const (
	DomainAvia           Domain = "avia"
	DomainRail           Domain = "rail"
	DomainBus            Domain = "bus"
	DomainEtrain         Domain = "etrain"
	DomainHotels         Domain = "hotels"
	DomainMultitransport Domain = "multitransport"
)

const (
	ResourceHelpOverview        = "tutu://help/overview"
	ResourceGeo                 = "tutu://geo"
	ResourceAmenitiesDictionary = "tutu://amenities/dictionary"
	ResourceStatus              = "tutu://status"
	ResourceSpecialOffers       = "tutu://special-offers"
	ResourceVersion             = "tutu://version"
)

const (
	KindDeeplink = "deeplink"

	KindSearchRedirect = "search_redirect"
)
