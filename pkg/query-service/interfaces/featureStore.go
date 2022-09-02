package interfaces

import (
	"go.signoz.io/query-service/model"
)

type FeatureStore interface {
	CheckFeature(orgID string, f model.featureKey) error
}
