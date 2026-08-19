package domain

var themeBadges = map[string][]string{ //nolint:gochecknoglobals // static vocabulary map.
	"nature":       {"лес", "озёра", "горы", "походы", "большая река", "вулканы", "степь"},
	"architecture": {"архитектура", "старый город", "юнеско", "кремль", "монастыри", "деревянное зодчество", "усадьбы"},
	"food":         {"гастрономия", "крафт", "вино", "национальная кухня"},
	"history":      {"музеи", "кремль", "старый город", "монастыри", "военная история", "усадьбы", "юнеско"},
	"events":       {"фестивали", "большие арены", "театры", "ночная жизнь"},
	"calm":         {"санатории", "лес", "озёра", "на выходные", "бюджетно"},
	"active":       {"походы", "горные лыжи", "сплавы", "велопрогулки", "горы"},
	"unusual":      {"вулканы", "северное сияние", "термальные источники", "стрит-арт", "деревянное зодчество"},
}

func BadgesForThemes(themes []string) []string {
	seen := make(map[string]struct{})
	badges := make([]string, 0, len(themes)*4)

	for _, theme := range themes {
		for _, badge := range themeBadges[theme] {
			if _, duplicate := seen[badge]; duplicate {
				continue
			}

			seen[badge] = struct{}{}
			badges = append(badges, badge)
		}
	}

	return badges
}
