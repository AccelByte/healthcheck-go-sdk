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

import "sync"

type healthDependency struct {
	Name           string `json:"name"`
	URL            string `json:"url"`
	Healthy        bool   `json:"healthy"`
	HardDependency bool   `json:"hardDependency"`
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
