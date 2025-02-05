/*
   Copyright 2021 The Kubermatic Kubernetes Platform contributors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package harbor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/kubermatic-labs/registryman/pkg/globalregistry"
)

const scannersPath = "/api/v2.0/scanners"

type scannerRegistration struct {
	Disabled         bool   `json:"disabled,omitempty"`
	Vendor           string `json:"vendor,omitempty"`
	Description      string `json:"description,omitempty"`
	Url              string `json:"url,omitempty"`
	Adapter          string `json:"adapter,omitempty"`
	AccessCredential string `json:"access_credential,omitempty"`
	Uuid             string `json:"uuid,omitempty"`
	Auth             string `json:"auth,omitempty"`
	IsDefault        bool   `json:"is_default,omitempty"`
	Version          string `json:"version,omitempty"`
	Health           string `json:"health,omitempty"`
	UseInternalAddr  bool   `json:"use_internal_addr,omitempty"`
	SkipCertVerify   bool   `json:"skip_cert_verify,omitempty"`
	Name             string `json:"name,omitempty"`
}

type scannerRegistrationRequest struct {
	Name             string `json:"name,omitempty"`
	Url              string `json:"url,omitempty"`
	AccessCredential string `json:"access_credential,omitempty"`
	Auth             string `json:"auth,omitempty"`
	Disabled         bool   `json:"disabled,omitempty"`
	UseInternalAddr  bool   `json:"use_internal_addr,omitempty"`
	SkipCertVerify   bool   `json:"skip_cert_verify,omitempty"`
	Description      string `json:"description,omitempty"`
}

type projectScanner struct {
	Uuid string `json:"uuid,omitempty"`
}
type scannerAPI struct {
	reg *registry
}

var _ globalregistry.Scanner = &scannerRegistrationRequest{}

func newScannerAPI(reg *registry) *scannerAPI {
	return &scannerAPI{
		reg: reg,
	}
}

func (s *scannerAPI) create(config globalregistry.Scanner) (string, error) {
	url := *s.reg.parsedUrl
	url.Path = scannersPath

	reqBodyBuf := bytes.NewBuffer(nil)
	err := json.NewEncoder(reqBodyBuf).Encode(&scannerRegistrationRequest{
		Name: config.GetName(),
		Url:  config.GetURL(),
	})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequest(http.MethodPost, url.String(), reqBodyBuf)
	if err != nil {
		return "", err
	}

	req.Header["Content-Type"] = []string{"application/json"}
	req.SetBasicAuth(s.reg.GetUsername(), s.reg.GetPassword())
	resp, err := s.reg.do(req)
	if err != nil {
		return "", err
	}

	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		return "", fmt.Errorf("scanner creation failed, %w", globalregistry.ErrRecoverableError)
	}

	scannerID := strings.TrimPrefix(
		resp.Header.Get("Location"),
		fmt.Sprintf("%s/", scannersPath))
	if err != nil {
		s.reg.logger.Error(err, "cannot parse scanner URL from response Location header",
			"location-header", resp.Header.Get("Location"))
		return "", err
	}
	return scannerID, nil
}

func (s *scannerAPI) getScannerIDByNameOrCreate(targetScanner globalregistry.Scanner) (string, error) {
	retrievedID := ""
	currentScanners, err := s.List()
	if err != nil {
		return "", err
	}

	for _, scannerIterator := range currentScanners {
		if strings.EqualFold(scannerIterator.GetName(), targetScanner.GetName()) {
			retrievedID = scannerIterator.(*scanner).id
		}
	}

	if err == nil && retrievedID != "" {
		return retrievedID, nil
	}

	s.reg.logger.V(1).Info("id not found, comparing with existing scanner registrations", "name", targetScanner.GetName())
	for _, scannerIterator := range currentScanners {
		if (strings.EqualFold(scannerIterator.GetName(), targetScanner.GetName()) ||
			strings.EqualFold(scannerIterator.GetURL(), targetScanner.GetURL())) &&
			!(strings.EqualFold(scannerIterator.GetName(), targetScanner.GetName()) &&
				strings.EqualFold(scannerIterator.GetURL(), targetScanner.GetURL())) {

			s.reg.logger.V(1).Info("updating existing scanner", scannerIterator.GetName(), targetScanner.GetName())
			err = s.update(scannerIterator.(*scanner).id, targetScanner)
			return scannerIterator.(*scanner).id, err
		}
	}

	s.reg.logger.V(1).Info("creating global scanner", "name", targetScanner.GetName())

	newScannerConfig := &scannerRegistrationRequest{
		Name:     targetScanner.GetName(),
		Url:      targetScanner.GetURL(),
		Disabled: false,
	}

	return s.create(newScannerConfig)
}

func (s *scannerAPI) List() ([]globalregistry.Scanner, error) {
	url := *s.reg.parsedUrl
	url.Path = scannersPath
	req, err := http.NewRequest(http.MethodGet, url.String(), nil)
	if err != nil {
		return nil, err
	}

	req.SetBasicAuth(s.reg.GetUsername(), s.reg.GetPassword())
	resp, err := s.reg.do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	scannerResult := []*scannerRegistration{}
	err = json.NewDecoder(resp.Body).Decode(&scannerResult)

	if err != nil {
		s.reg.logger.Error(err, "json decoding failed")
		b := bytes.NewBuffer(nil)
		_, err := b.ReadFrom(resp.Body)
		if err != nil {
			panic(err)
		}
		s.reg.logger.Info(b.String())
	}

	scanners := make([]globalregistry.Scanner, 0)

	if len(scannerResult) == 0 {
		return scanners, err
	}

	for _, scannerIterator := range scannerResult {
		scanners = append(scanners, &scanner{
			id:        scannerIterator.Uuid,
			api:       s,
			name:      scannerIterator.Name,
			url:       scannerIterator.Url,
			isDefault: scannerIterator.IsDefault,
		})
	}
	return scanners, err
}

func (s *scannerAPI) SetForProject(projectID int, scannerID string) error {
	url := *s.reg.parsedUrl
	url.Path = fmt.Sprintf("%s/%d/scanner", path, projectID)

	reqBodyBuf := bytes.NewBuffer(nil)
	err := json.NewEncoder(reqBodyBuf).Encode(&projectScanner{
		Uuid: scannerID,
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPut, url.String(), reqBodyBuf)
	if err != nil {
		return err
	}

	req.SetBasicAuth(s.reg.GetUsername(), s.reg.GetPassword())
	resp, err := s.reg.do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("failed to set scanner for project-id:%d, %w", projectID, globalregistry.ErrRecoverableError)
	}
	return err
}

func (s *scannerAPI) getForProject(id int) (globalregistry.Scanner, error) {
	url := *s.reg.parsedUrl
	url.Path = fmt.Sprintf("%s/%d/scanner", path, id)
	req, err := http.NewRequest(http.MethodGet, url.String(), nil)
	if err != nil {
		return nil, err
	}

	req.SetBasicAuth(s.reg.GetUsername(), s.reg.GetPassword())
	resp, err := s.reg.do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	scannerResult := &scannerRegistration{}
	err = json.NewDecoder(resp.Body).Decode(scannerResult)

	if err != nil {
		s.reg.logger.Error(err, "json decoding failed")
		b := bytes.NewBuffer(nil)
		_, err := b.ReadFrom(resp.Body)
		if err != nil {
			panic(err)
		}
		s.reg.logger.Info(b.String())
	}

	resultScanner := &scanner{
		id:   scannerResult.Uuid,
		api:  s,
		name: scannerResult.Name,
		url:  scannerResult.Url,
	}
	return resultScanner, err

}

func (s *scannerAPI) update(id string, targetScanner globalregistry.Scanner) error {
	url := *s.reg.parsedUrl
	url.Path = fmt.Sprintf("%s/%s", scannersPath, id)

	reqBodyBuf := bytes.NewBuffer(nil)
	err := json.NewEncoder(reqBodyBuf).Encode(&scannerRegistrationRequest{
		Name: targetScanner.GetName(),
		Url:  targetScanner.GetURL(),
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPut, url.String(), reqBodyBuf)
	if err != nil {
		return err
	}

	req.Header["Content-Type"] = []string{"application/json"}
	req.SetBasicAuth(s.reg.GetUsername(), s.reg.GetPassword())

	resp, err := s.reg.do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("failed to update scanner, %w", globalregistry.ErrRecoverableError)
	}
	return err
}

func (s *scannerAPI) delete(id string) error {
	url := *s.reg.parsedUrl
	url.Path = fmt.Sprintf("%s/%s", scannersPath, id)

	req, err := http.NewRequest(http.MethodDelete, url.String(), nil)
	if err != nil {
		return err
	}

	req.Header["Content-Type"] = []string{"application/json"}
	req.SetBasicAuth(s.reg.GetUsername(), s.reg.GetPassword())

	resp, err := s.reg.do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("failed to remove scanner, %w", globalregistry.ErrRecoverableError)
	}
	return err
}

func (c *scannerRegistrationRequest) GetName() string {
	return c.Name
}

func (c *scannerRegistrationRequest) GetAuth() string {
	return c.Auth
}

func (c *scannerRegistrationRequest) GetCredential() string {
	return c.AccessCredential
}

func (c *scannerRegistrationRequest) GetURL() string {
	return c.Url
}

func (c *scannerRegistrationRequest) IsDisabled() bool {
	return c.Disabled
}

func (c *scannerRegistrationRequest) GetDescription() string {
	return c.Description
}
