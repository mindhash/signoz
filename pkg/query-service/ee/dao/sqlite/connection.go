package sqlite

import (
	"context"
	"github.com/pkg/errors"
	baseDao "go.signoz.io/query-service/dao"
	baseDaoSqlite "go.signoz.io/query-service/dao/sqlite"
	"go.signoz.io/query-service/model"
)

type modelDao struct {
	*baseDaoSqlite.ModelDaoSqlite
}

// InitDB creates and extends base model DB repository
func InitDB(dataSourceName string) (*modelDao, error) {
	dao, err := baseDaoSqlite.InitDB(dataSourceName)
	if err != nil {
		return nil, err
	}
	// set package variable so dependent base methods (e.g. AuthCache)  will work
	baseDao.SetDB(dao)
	return &modelDao{
		dao,
	}, nil
}

// RegisterOrFetchSAMLUser gets or creates a new user from a SAML response
func (m *modelDao) FetchOrRegisterSAMLUser(email, firstname, lastname string) (*model.UserPayload, error) {
	userPayload, err := m.GetUserByEmail(context.Background(), email)
	if err != nil {
		return nil, errors.Wrap(err.Err, "user not found")
	}
	// todo(amol) : Need to implement user creation
	return userPayload, nil
}
