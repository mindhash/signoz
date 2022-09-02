package signoz

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go.signoz.io/query-service/model"
	eeModel "go.signoz.io/query-service/model"
	"go.uber.org/zap"
	"net/http"
)

type LicensePayload struct{
	eeModel.License
	
}

type ServiceAccount struct {
}

func NewServiceAccount() *ServiceAccount {
	return &ServiceAccount{}
}

// todo: what happens when users delete orgID and create a new one
// ApplyLicense sends key to license.signoz.io and gets activation data
func (s *ServiceAccount) ApplyLicense(key, orgId string) (*eeModel.License, model.ApiError) {
	license := eeModel.License{}
	licenseReq := map[string]string{
		"key":   key,
		"orgId": orgId,
	}

	reqString, _ := json.Marshal(licenseReq)
	response, err := http.Post("https://license.signoz.io/license/apply", "application/json", bytes.NewBuffer(reqString))

	if err != nil {
		zap.S().Errorf("msg: ", "failed to fetch licensing details", "\t err:", err)
	license := eeModel.License{}
	return nil, &model.ApiError{Typ: model.ErrorInternal, Err: err}
	}

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		zap.S().Errorf("msg:", "failed to read response from license API", "\t err:", err)
		return nil, &model.ApiError{Typ: model.ErrorInternal, Err: err}
	}

	defer response.Body.Close()

	if (response.StatusCode == 200) {
		err := json.Marshal(body, &license)
		if err != nil {
			zap.S().Errorf("msg:", "failed to read response from license api", "\t err:", err)
			return nil, &model.ApiError{Typ: model.ErrorInternal, Err: err}
		}
	}

	if response.StatusCode > 400 && response.StatusCode < 500 {
		zap.S().Errorf("msg:", "bad request when calling license api", "\t err:", string(body)))
		return nil, &model.ApiError{Typ: model.ErrorBadData, Err: fmt.Errorf(string(body))}
	}

	if response.StatusCode > 500 {
		zap.S().Errorf("msg:", "internal error when calling license api", "\t err:", string(body)))
		return license, &model.ApiError{Typ: model.ErrorInternal, Err: string(body)}
	}

	return license, nil
}
