package license

import (
	"github.com/jmoiron/sqlx"

	"github.com/google/uuid"
	licenseSqlite "go.signoz.io/query-service/ee/license/sqlite"
	"go.signoz.io/query-service/model"
	"go.uber.org/zap"
)

// Repo is license repo. stores license keys in a secured DB
type Repo struct {
	db *sqlx.DB
}

// NewLicenseRepo initiates a new license repo
func NewLicenseRepo(db *sqlx.DB) Repo {
	return Repo{
		db: db,
	}
}

func (r *Repo) InitDB(engine string) error {
	switch engine {
	case "sqlite3", "sqlite":
		return licenseSqlite.InitDB(r.db)
	default:
		return fmt.Errorf("unsupported db")
	}
}

// GetLicenses fetches all active licenses from DB
func (r *Repo) GetActiveLicenses() ([]License, error) {
	var err error
	licenses := []License{}

	// we want end date nulls to appear last and start date
	query := "SELECT * FROM licenses WHERE $1 BETWEEN start and end order by end IS NULL, start"

	err = r.db.Select(&licenses, query, time.Now())
	if err != nil {
		return nil, &model.ApiError{Typ: model.ErrorExec, Err: err}
	}

	return licenses, nil
}

// CreateDashboard creates a new dashboard
func InsertLicense(ctx context.Context, key string, org *model.Organization) (*License, error) {

	l := &License{}
	if key == "" {
		return nil, fmt.Errorf("insert license failed: license key is required")
	}

	if org == nil {
		return nil, fmt.Errorf("insert license failed: org is required")
	}

	l.CreatedAt = time.Now()
	l.UpdatedAt = time.Now()
	l.UUID = uuid.New().String()
	l.OrgID = org.ID

	query := "INSERT INTO licenses (uuid, created_at, updated_at, key, org_id) VALUES ($1, $2, $3, $4, $5)"

	err := db.ExecContext(ctx, query, l.UUID, l.CreatedAt, l.UpdatedAt, l.Key, l.OrgID)

	if err != nil {
		zap.S().Errorf("Error in inserting license data: ", license, err)
		return nil, fmt.Errorf("failed to insert license in db: %v", err)
	}

	return l, nil
}
