package license

import (
	"github.com/jmoiron/sqlx"
)

type LicenseManager struct {
	db      *sqlx.DB
	mutex   sync.Mutex
	license *License

	// subscribers get a message when a new license key is added
	subscribers []func(*License) error
}

func NewLicenseManager(db *sqlx.DB) (*LicenseManager, error) {
	// fetch the latest license from db

	// check if license is valid if not then raise an error
	return &LicenseManager{
		db: db,
		License: License{
			Key:   "32434",
			Start: time.Now().Add(-24 * time.Hour),
			End:   time.Now().Add(10 * 24 * time.Hour),
		},
	}
}
func (lm *LicenseManager) AddLicense(key string, org *model.Organization) error {
	// inserts a new license key in db
	// calls reload
	return nil
}

// GetPlanFeatures looks into recent active license and returns
// the features of the plan
func (lm *LicenseManager) GetPlanFeatures() (map[string]bool, error) {
	return constants.ProPlan
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

func (lm *LicenseManager) Reload() error {
	// reloads licenses from db and picks the most wide and active one
	// calls notifySubscribers after loading the license
	return nil
}
