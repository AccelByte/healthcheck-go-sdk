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
	"net/http"
	"strings"
	"sync"

	restfulV1 "github.com/emicklei/go-restful"
	"github.com/emicklei/go-restful/v3"
	"github.com/sirupsen/logrus"
)

const (
	nameURLKeySeparator    = "|"
	defaultHealthCheckPath = "/healthz"

	softDependency = "soft"
	hardDependency = "hard"
)

type handler struct {
	serviceName      string
	basePath         string
	checksMutex      sync.RWMutex
	healthDependency map[string]CheckFunc
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
}

func New(serviceName, basePath string) Handler {
	return &handler{
		serviceName:      serviceName,
		basePath:         basePath,
		checksMutex:      sync.RWMutex{},
		healthDependency: make(map[string]CheckFunc),
	}
}

// AddHealthCheck adds a dependency health check. It will be a soft dependency check, hence if the check failed,
// it will only return healthy=false on the corresponding dependency and will not affect the overall healthy status.
func (h *handler) AddHealthCheck(name, url string, check CheckFunc) {
	h.checksMutex.Lock()
	defer h.checksMutex.Unlock()

	key := name + nameURLKeySeparator + url + nameURLKeySeparator + softDependency
	h.healthDependency[key] = check
}

// AddHardHealthCheck adds a dependency hard health check.
// It will return healthy=false on the corresponding dependency and the overall healthy status.
func (h *handler) AddHardHealthCheck(name, url string, check CheckFunc) {
	h.checksMutex.Lock()
	defer h.checksMutex.Unlock()

	key := name + nameURLKeySeparator + url + nameURLKeySeparator + hardDependency
	h.healthDependency[key] = check
}

func (h *handler) AddWebservice() []*restful.WebService {
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

func (h *handler) AddWebserviceV1() []*restfulV1.WebService {
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

// nolint: gomnd
func (h *handler) runChecks(result *response) {
	h.checksMutex.Lock()
	defer h.checksMutex.Unlock()

	var wg sync.WaitGroup

	for nameURL, check := range h.healthDependency {
		wg.Add(1)

		nameURL := nameURL
		check := check
		res := strings.Split(nameURL, nameURLKeySeparator)
		dependencyName := res[0]
		dependencyURL := res[1]
		isHardDependency := res[2] == hardDependency

		go func() {
			defer wg.Done()

			isHealthy := true

			if err := check(); err != nil {
				logrus.Error(err)

				isHealthy = false
			}

			result.appendHealthCheckDependency(
				healthDependency{
					Name:           dependencyName,
					Healthy:        isHealthy,
					URL:            dependencyURL,
					HardDependency: isHardDependency,
				})
		}()
	}

	wg.Wait()
}

func (h *handler) getResponse() (int, *response) {
	otherComponents := make([]healthOtherComponent, 0)
	healthStatus := &response{
		Name:    h.serviceName,
		Healthy: true,
		Others:  otherComponents,
	}

	h.runChecks(healthStatus)

	responseStatus := http.StatusOK

	for _, dependency := range healthStatus.Dependencies {
		if !dependency.Healthy && dependency.HardDependency {
			responseStatus = http.StatusServiceUnavailable
			healthStatus.Healthy = false

			break
		}
	}

	return responseStatus, healthStatus
}

// handlerV3 will support for go-restful v3
func (h *handler) handlerV3(_ *restful.Request, resp *restful.Response) {
	responseStatus, healthStatus := h.getResponse()

	if err := resp.WriteHeaderAndJson(responseStatus, healthStatus, restful.MIME_JSON); err != nil {
		logrus.Error("Error " + err.Error())
	}
}

// handlerV1 will support for go-restful v1
func (h *handler) handlerV1(_ *restfulV1.Request, resp *restfulV1.Response) {
	responseStatus, healthStatus := h.getResponse()

	if err := resp.WriteHeaderAndJson(responseStatus, healthStatus, restful.MIME_JSON); err != nil {
		logrus.Error("Error " + err.Error())
	}
}
