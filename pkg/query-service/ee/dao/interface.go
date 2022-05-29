package dao

import (
	"github.com/jmoiron/sqlx"
	baseDao "go.signoz.io/query-service/dao"
	"go.signoz.io/query-service/model"
)

type ModelDao interface {
	baseDao.ModelDao
	DB() *sqlx.DB
	FetchOrRegisterSAMLUser(email, firstname, lastname string) (*model.UserPayload, error)
}
