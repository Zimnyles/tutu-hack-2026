package scoring

import (
	"context"

	"github.com/tutu-hack/openworld/internal/models/domain"
)

type Settings interface {
	ScoringWeights(context.Context) (domain.ScoringWeights, error)
}
