// Copyright 2021 AccelByte Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package healthcheck

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"

	restfulV1 "github.com/emicklei/go-restful"
	"github.com/emicklei/go-restful/v3"
	"github.com/sirupsen/logrus"
)

const (
	defaultHealthCheckPath = "/healthz"

	DefaultBackgroundCheckInterval = 60 * time.Second
)

type healthCheck struct {
	serviceName       string
	basePath          string
	dependenciesMutex sync.RWMutex
	dependencies      map[string]healthDependency
	bgCheckRunning    bool
	bgCheckInterval   time.Duration
}

type Config struct {
	ServiceName             string
	BasePath                string
	BackgroundCheckInterval time.Duration
}

type Handler interface {
	AddWebservice() []*restful.WebService
	AddWebserviceV1() []*restfulV1.WebService

	// AddHealthCheck adds a dependency health check. It will be a soft dependency check, hence if the check failed,
	// it will only return healthy=false on the corresponding dependency and will not affect the overall healthy status.
	AddHealthCheck(name, url string, check CheckFunc)

	// AddHardHealthCheck adds a hard dependency health check.
	// It will return healthy=false on the corresponding dependency and the overall healthy status.
	AddHardHealthCheck(name, url string, check CheckFunc)

	// StartBackgroundCheck starts a background health check worker. The health check will be performed at a
	// certain interval, specified in Config, rather than every health endpoint request.
	StartBackgroundCheck(ctx context.Context)

	// UpdateHealth updates a dependency health status. If you want to exclusively update a dependency health
	// using this, make sure to pass nil value onto check function param when adding the dependency using
	// AddHealthCheck or AddHardHealthCheck.
	UpdateHealth(name string, isHealthy bool, checkError *CheckError) error
}

func New(config *Config) Handler {
	if config.BackgroundCheckInterval <= 0 {
		config.BackgroundCheckInterval = DefaultBackgroundCheckInterval
	}

	return &healthCheck{
		serviceName:       config.ServiceName,
		basePath:          config.BasePath,
		dependenciesMutex: sync.RWMutex{},
		dependencies:      make(map[string]healthDependency),
		bgCheckInterval:   config.BackgroundCheckInterval,
	}
}

// AddHealthCheck adds a dependency health check. It will be a soft dependency check, hence if the check failed,
// it will only return healthy=false on the corresponding dependency and will not affect the overall healthy status.
func (h *healthCheck) AddHealthCheck(name, url string, check CheckFunc) {
	h.dependenciesMutex.Lock()
	defer h.dependenciesMutex.Unlock()

	h.dependencies[name] = healthDependency{
		Name:      name,
		URL:       url,
		checkFunc: check,
		LastError: nil,
	}
}

// AddHardHealthCheck adds a dependency hard health check.
// It will return healthy=false on the corresponding dependency and the overall healthy status.
func (h *healthCheck) AddHardHealthCheck(name, url string, check CheckFunc) {
	h.dependenciesMutex.Lock()
	defer h.dependenciesMutex.Unlock()

	h.dependencies[name] = healthDependency{
		Name:           name,
		URL:            url,
		HardDependency: true,
		checkFunc:      check,
		LastError:      nil,
	}
}

// UpdateHealth updates a dependency health status.
func (h *healthCheck) UpdateHealth(name string, isHealthy bool, checkError *CheckError) error {
	h.dependenciesMutex.Lock()
	defer h.dependenciesMutex.Unlock()

	dependency, exist := h.dependencies[name]
	if !exist {
		return errors.New("dependency name does not exist")
	}
	dependency.Healthy = isHealthy
	now := time.Now()
	dependency.LastCall = &now
	if isHealthy {
		dependency.LastKnownGoodCall = dependency.LastCall
	}
	if checkError != nil {
		dependency.LastError = &lastError{Message: checkError.Message}
		if !checkError.Timestamp.IsZero() {
			dependency.LastError.Timestamp = &checkError.Timestamp
		}
	}
	h.dependencies[name] = dependency

	return nil
}

func (h *healthCheck) AddWebservice() []*restful.WebService {
	webservices := make([]*restful.WebService, 2)

	webservice := new(restful.WebService)

	webservice.Path(defaultHealthCheckPath)
	// route to http://example.com/healthz
	webservice.Route(
		webservice.GET("").
			To(h.handlerV3).
			Operation("GetHealthcheckInfo"))

	webservices[0] = webservice

	webserviceWithBasePath := new(restful.WebService)
	webserviceWithBasePath.Path(h.basePath + defaultHealthCheckPath)
	// route to http://example.com/basepath/healthz
	webserviceWithBasePath.Route(
		webserviceWithBasePath.GET("").
			To(h.handlerV3).
			Operation("GetHealthcheckInfoV1"))

	webservices[1] = webserviceWithBasePath

	return webservices
}

func (h *healthCheck) AddWebserviceV1() []*restfulV1.WebService {
	webservices := make([]*restfulV1.WebService, 2)

	webservice := new(restfulV1.WebService)
	webservice.Path(defaultHealthCheckPath)
	// route to http://example.com/healthz
	webservice.Route(webservice.GET("").To(h.handlerV1))
	webservices[0] = webservice

	webserviceWithBasePath := new(restfulV1.WebService)
	webserviceWithBasePath.Path(h.basePath + defaultHealthCheckPath)
	// route to http://example.com/basepath/healthz
	webserviceWithBasePath.Route(webserviceWithBasePath.GET("").To(h.handlerV1))
	webservices[1] = webserviceWithBasePath

	return webservices
}

func (h *healthCheck) StartBackgroundCheck(ctx context.Context) {
	if h.bgCheckRunning {
		return
	}
	h.bgCheckRunning = true
	h.runChecks()

	ticker := time.NewTicker(h.bgCheckInterval)

	go func() {
		for {
			select {
			case <-ticker.C:
				h.runChecks()
			case <-ctx.Done():
				ticker.Stop()
				h.bgCheckRunning = false
				logrus.Info("Background health check worker stopped")
				return
			}
		}
	}()
}

// nolint: gomnd
func (h *healthCheck) runChecks() {
	wg := &sync.WaitGroup{}
	for _, d := range h.dependencies {
		wg.Add(1)
		go h.check(wg, d)
	}

	wg.Wait()
}

func (h *healthCheck) check(wg *sync.WaitGroup, d healthDependency) {
	defer wg.Done()
	h.dependenciesMutex.Lock()
	defer h.dependenciesMutex.Unlock()
	d.check()
	h.dependencies[d.Name] = d
}

func (h *healthCheck) getResponse() (int, *response) {
	otherComponents := make([]healthOtherComponent, 0)
	healthStatusResp := &response{
		Name:    h.serviceName,
		Healthy: true,
		Others:  otherComponents,
	}

	// if background health check worker is not running, check immediately
	if !h.bgCheckRunning {
		h.runChecks()
	}

	h.dependenciesMutex.Lock()
	for _, v := range h.dependencies {
		healthStatusResp.appendHealthCheckDependency(v)
	}
	h.dependenciesMutex.Unlock()

	responseStatus := http.StatusOK

	for _, dependency := range healthStatusResp.Dependencies {
		if !dependency.Healthy && dependency.HardDependency {
			responseStatus = http.StatusServiceUnavailable
			healthStatusResp.Healthy = false
			break
		}
	}

	return responseStatus, healthStatusResp
}

// handlerV3 will support for go-restful v3
func (h *healthCheck) handlerV3(_ *restful.Request, resp *restful.Response) {
	responseStatus, healthStatus := h.getResponse()

	if err := resp.WriteHeaderAndJson(responseStatus, healthStatus, restful.MIME_JSON); err != nil {
		logrus.Error("Error " + err.Error())
	}
}

// handlerV1 will support for go-restful v1
func (h *healthCheck) handlerV1(_ *restfulV1.Request, resp *restfulV1.Response) {
	responseStatus, healthStatus := h.getResponse()

	if err := resp.WriteHeaderAndJson(responseStatus, healthStatus, restful.MIME_JSON); err != nil {
		logrus.Error("Error " + err.Error())
	}
}
