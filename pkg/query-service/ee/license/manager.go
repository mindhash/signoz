package license

import (
	"github.com/jmoiron/sqlx"
	"go.signoz.io/query-service/constants"
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
	err := repo.InitDB()

	if err != nil {
		return fmt.Errorf("failed to initiate license repo: %v", err)
	}
	m := &LicenseManager{
		repo: repo,
	}

	if err := m.LoadLicenses(); err != nil {
		return fmt.Errorf("failed to load licenses from db: %v", err)
	}

	return m
}

func (lm *LicenseManager) LoadLicenses() error {
	// read license from db
	activeLicenses, err := lm.repo.GetLicenses()
	lm := make(map[string]*License, 0)
	for l, i := range activeLicenses {
		// todo(amol): skip repeated ones
		lm[l.orgID] = l
	}

	lm.mutex.Lock()
	defer lm.mutex.Unlock()
	lm.licenseCache = lm
}

func (lm *LicenseManager) AddLicense(key string, org *model.Organization) error {
	// org is mandatory or raise an error
	if org == nil {
		return fmt.Println("org is required when adding a license key")
	}

	// todo(amol): validate and extract license key using  public signature

	// inserts a new license key in db
	l, err := lm.InsertLicense(key, org)
	if err != nil {
		return err
	}

	// extract supported features from encrypted license key
	features := l.LoadFeatures()
	// update cache
	lm.mutex.Lock()
	defer lm.mutex.Unlock()
	licenseCache[org.ID] = l
	featureCache[org.ID] = features

	return nil
}

// CheckFeature returns true when feature is available to the
// target org
func (lm *LicenseManager) CheckFeature(orgID string, s string) bool {

	if features, ok := lm.featureCache[orgID]; ok {
		if _, ok := features[s]; ok {
			return true
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
func (lm *LicenseManager) notifySubscribers() error {
	for _, s := range lm.subscribers {
		if err := s(lm.License); err != nil {
			return err
		}
	}

	return nil
}
