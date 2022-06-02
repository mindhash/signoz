package license

import (
	"context"
	"fmt"
	"github.com/jmoiron/sqlx"
	"time"

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
func (r *Repo) GetActiveLicenses(ctx context.Context) ([]License, error) {
	var err error
	licenses := []License{}

	// we want end date nulls to appear last and start date
	query := "SELECT key, start, end, org_id, created_at, uuid FROM licenses WHERE $1 BETWEEN start and end order by end IS NULL, start"

	err = r.db.Select(&licenses, query, time.Now())
	if err != nil {
		return nil, fmt.Errorf("failed to get active licenses from db: %v", err)
	}

	return licenses, nil
}

// CreateDashboard creates a new dashboard
func (r *Repo) InsertLicenseTx(ctx context.Context, key string, org *model.Organization) (*License, *sqlx.Tx, error) {
	if key == "" {
		return nil, nil, fmt.Errorf("insert license failed: license key is required")
	}
	if org == nil {
		return nil, nil, fmt.Errorf("insert license failed: org is required")
	}

	l := &License{Key: key}

	l.CreatedAt = time.Now()
	l.UUID = uuid.New().String()
	l.OrgId = org.Id

	query := "INSERT INTO licenses (uuid, created_at, key, org_id) VALUES ($1, $2, $3, $4)"
	tx, err := r.db.Beginx()
	if err != nil {
		zap.S().Errorf("Error in inserting license data: ", l, err)
		return nil, nil, fmt.Errorf("failed to initiate dbtx while inserting license in db: %v", err)
	}

	_, err = tx.ExecContext(ctx, query, l.UUID, l.CreatedAt, l.Key, l.OrgId)

	if err != nil {
		zap.S().Errorf("Error in inserting license data: ", l, err)
		return nil, nil, fmt.Errorf("failed to insert license in db: %v", err)
	}

	return l, tx, nil
}
