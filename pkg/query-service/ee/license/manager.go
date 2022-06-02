package license

import (
	"context"
	"fmt"
	"github.com/jmoiron/sqlx"
	"go.signoz.io/query-service/constants"
	"go.signoz.io/query-service/model"
	"sync"
)

type LicenseManager struct {
	repo  *Repo
	mutex sync.Mutex

	// license cache
	licenseCache map[string]*License

	// cache of features by org id
	// load happens on first time accesss
	// refreshes when billing plan changes (e.g. upgrade to pro)
	featureCache map[string]constants.SupportedFeatures

	// subscribers get a message when a new license key is added
	subscribers []func(*License) error
}

func NewLicenseManager(db *sqlx.DB) (*LicenseManager, error) {
	// fetch the latest license from db
	repo := NewLicenseRepo(db)
	err := repo.InitDB("sqlite3")

	if err != nil {
		return nil, fmt.Errorf("failed to initiate license repo: %v", err)
	}
	m := &LicenseManager{
		repo: &repo,
	}

	if err := m.LoadLicenses(); err != nil {
		return nil, fmt.Errorf("failed to load licenses from db: %v", err)
	}

	return m, nil
}

func (lm *LicenseManager) LoadLicenses() error {
	// read license from db
	activeLicenses, err := lm.repo.GetActiveLicenses(context.Background())
	if err != nil {
		return err
	}
	lmap := make(map[string]*License, 0)
	fmap := make(map[string]constants.SupportedFeatures, 0)
	for _, l := range activeLicenses {

		// load features for the license
		features, err := l.LoadFeatures()
		if err != nil {
			return fmt.Errorf("failed to load features for the given license: %v", err)
		}

		// todo(amol): pick from repeated licenses ones
		lmap[l.OrgId] = &l
		fmap[l.OrgId] = features

	}

	lm.mutex.Lock()
	defer lm.mutex.Unlock()
	lm.licenseCache = lmap
	lm.featureCache = fmap
	return nil
}

func (lm *LicenseManager) ApplyLicense(key string, org *model.Organization) (err error) {
	ctx := context.Background()
	if org == nil {
		return fmt.Errorf("org is required when adding a license key")
	}

	// todo(amol): validate and extract license key using  public signature

	// inserts a new license key in db
	l, tx, err := lm.repo.InsertLicenseTx(ctx, key, org)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		} else {
			tx.Commit()
		}

	}()

	// extract supported features from encrypted license key
	features, err := l.LoadFeatures()
	if err != nil {
		return fmt.Errorf("failed to load features for the given license: %v", err)
	}

	// update cache
	lm.mutex.Lock()
	defer lm.mutex.Unlock()
	lm.licenseCache[org.Id] = l
	lm.featureCache[org.Id] = features

	return nil
}

// GetFeatureFlags returns feature flags for a given org license
func (lm *LicenseManager) GetFeatureFlags(orgId string) (constants.SupportedFeatures, error) {
	fmt.Println("licenseCache:", lm.licenseCache)
	fmt.Println("feature cache:", lm.featureCache)
	if l, ok := lm.licenseCache[orgId]; ok {
		fmt.Println("license found:", l)
		// todo(amol): validate license is within dates

		if features, ok := lm.featureCache[orgId]; !ok {

			return l.LoadFeatures()
		} else {
			fmt.Println("features found:", features)
			return features, nil
		}
	}

	return constants.BasicPlan, nil
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

// Subscribe sends a signal when license plan changes
func (lm *LicenseManager) Subscribe(ss ...func(*License) error) {
	lm.mutex.Lock()
	defer lm.mutex.Unlock()

	lm.subscribers = append(lm.subscribers, ss...)
}

// notifySubscribers will be used to notify subscribers when
// license config changes
func (lm *LicenseManager) notifySubscribers(l *License) error {
	for _, s := range lm.subscribers {
		if err := s(l); err != nil {
			return err
		}
	}

	return nil
}
