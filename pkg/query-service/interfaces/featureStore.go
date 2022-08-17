package interfaces

import (
  "signoz.io/query-service/model"
)

type FeatureStore interface {
  CanAccess(orgID string, f model.featureKey)
}
