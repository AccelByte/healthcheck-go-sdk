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
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

type healthDependency struct {
	Name              string    `json:"name"`
	URL               string    `json:"url"`
	Healthy           bool      `json:"healthy"`
	HardDependency    bool      `json:"hardDependency"`
	LastKnownGoodCall time.Time `json:"lastKnownGoodCall"`
	LastCall          time.Time `json:"lastCall"`
	LastError         LastError `json:"lastError"`
	checkFunc         CheckFunc
}

type LastError struct {
	timestamp time.Time `json:"timestamp"`
	message   string    `json:"message"`
}

func (h *healthDependency) check() {
	if h.checkFunc == nil {
		return
	}

	h.LastCall = time.Now()
	err := h.checkFunc()
	if err != nil {
		logrus.Error(err)
		h.LastError.message = err.Error()
		h.LastError.timestamp = h.LastCall
		h.Healthy = false
		return
	}
	h.Healthy = true
	h.LastKnownGoodCall = time.Now()

	return
}

type CheckFunc func() error

// healthOtherComponent health status other component of service.
type healthOtherComponent struct {
	Name    string `json:"name"`
	Healthy bool   `json:"healthy"`
}

type response struct {
	lock sync.RWMutex

	Name         string                 `json:"name"`
	Healthy      bool                   `json:"healthy"`
	Dependencies []healthDependency     `json:"dependencies"`
	Others       []healthOtherComponent `json:"others"`
}

func (h *response) appendHealthCheckDependency(dependency healthDependency) {
	h.lock.Lock()
	defer h.lock.Unlock()
	h.Dependencies = append(h.Dependencies, dependency)
}
