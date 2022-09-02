package license

import (
	"context"
	"fmt"
	"github.com/jmoiron/sqlx"
	eeModel "go.signoz.io/query-service/ee/model"
	"go.signoz.io/query-service/model"
	"sync"
)

type Manager interface {
	AddLicense(key string, org *model.Organization) (*eeModel.License, *model.ApiError)
	GetLicenses(org *model.Organization) ([]eeModel.License, *model.ApiError)
}

type LicenseManager struct {
	repo  *Repo
	mutex sync.Mutex

	// cache of features by org id
	// load happens on first time accesss
	// refreshes when billing plan changes (e.g. upgrade to pro)
	featureCache map[string]model.PlanFeatures
}

func NewManager(db *sqlx.DB) (*LicenseManager, error) {
	// fetch the latest license from db
	repo := NewLicenseRepo(db)
	err := repo.InitDB("sqlite3")

	if err != nil {
		return nil, fmt.Errorf("failed to initiate license repo: %v", err)
	}
	m := &LicenseManager{
		repo: &repo,
	}

	if err := m.init(); err != nil {
		return m, err
	}

	return m, nil
}

func (lm *LicenseManager) init() error {
	if err := lm.LoadLicenses(); err != nil {
		return fmt.Errorf("failed to load licenses from db: %v", err)
	}
	return nil
}

func (lm *LicenseManager) CheckFeature(org *model.Organization, f model.FeatureKey) error {
	return nil
}

func (lm *LicenseManager) GetLicenses(org *model.Organization) ([]License, *model.ApiError) {
	return nil, nil
}

func (lm *LicenseManager) LoadLicenses() error {
	// read license from db
	activeLicenses, err := lm.repo.GetActiveLicenses(context.Background())
	if err != nil {
		return err
	}
	fmap := make(map[string]model.PlanFeatures, 0)
	for _, l := range activeLicenses {

		// call a go-routine to ping license to signoz server

		// load features for the license
		features, err := l.ParseFeatures()
		if err != nil {
			return fmt.Errorf("failed to load features for the given license: %v", err)
		}

		// todo(amol): pick from repeated licenses ones
		// todo(amol): do we append all the features for multiple active license?
		fmap[l.OrgId] = features
	}

	lm.mutex.Lock()
	defer lm.mutex.Unlock()
	lm.featureCache = fmap

	return nil
}

func (lm *LicenseManager) AddLicense(key string, org *model.Organization) *model.ApiError {
	ctx := context.Background()
	if org == nil {
		return &model.ApiError{
			Typ: model.ErrorBadData,
			Err: fmt.Errorf("org is required when adding a license key"),
		}
	}

	// call license.signoz.io to activate license

	// parse the response into license
	license := eeModel.License{Key: key}

	// inserts a new license key in db
	l, err := lm.repo.InsertLicense(ctx, &license, org)
	if err != nil {
		return &model.ApiError{
			Typ: model.ErrorInternal,
			Err: err,
		}
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		} else {
			tx.Commit()
		}

	}()

	// extract plan features from license
	features, err := l.ParseFeatures()
	if err != nil {
		return &model.ApiError{
			Typ: model.ErrorInternal,
			Err: fmt.Errorf("failed to load features for the given license: %v", err),
		}
	}

	// update cache
	lm.mutex.Lock()
	defer lm.mutex.Unlock()
	lm.featureCache[org.Id] = features

	return nil
}

// GetFeatureFlags returns feature flags for a given org license
func (lm *LicenseManager) GetFeatureFlags(orgId string) (model.PlanFeatures, error) {

	if features, ok := lm.featureCache[orgId]; !ok {
		return model.BasicPlan, nil
	} else {
		return features, nil
	}

	return model.BasicPlan, nil
}

// CheckFeature returns true when feature is available to the
// target org
func (lm *LicenseManager) CheckFeature(orgId string, s string) bool {
	// check license is valid
	if l, ok := lm.licenseCache[orgId]; ok {
		// todo(amol): validate license dates
		l = l
		// feature cache is loaded at the same time with license cache
		// so no need to fetch the features against the license
		if features, ok := lm.featureCache[orgId]; ok {
			if ok := features[s]; ok {
				return true
			}
		}
	}

	// cache hit failed, this means feature is not available. as
	// we load each license into the cache when server starts or when
	// license is added
	return false
}
