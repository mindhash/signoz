package db

import (
	"github.com/ClickHouse/clickhouse-go/v2"

	"github.com/jmoiron/sqlx"

	baseAppCh "go.signoz.io/query-service/app/clickhouseReader"
)

type clickhouseReader struct {
	ch   clickhouse.Conn
	qsdb *sqlx.DB
	*baseAppCh.ClickHouseReader
}

func NewClickhouseReader(localDB *sqlx.DB) *clickhouseReader {
	baseReader := baseAppCh.NewReader(localDB)
	return &clickhouseReader{
		ch:               baseReader.GetDB(),
		qsdb:             baseReader.GetRelationalDB(),
		ClickHouseReader: baseReader,
	}
}

func (r *clickhouseReader) Start() {
	r.ClickHouseReader.Start()
}
