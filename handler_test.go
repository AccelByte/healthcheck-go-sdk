package healthcheck

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	caller "github.com/AccelByte/http-test-caller"
	restfulV1 "github.com/emicklei/go-restful"
	"github.com/emicklei/go-restful/v3"
	"github.com/parnurzeal/gorequest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testURL     = "www.test.example.com"
	serviceName = "test"
	servicePath = "/service"
)

func Test_endpoint(t *testing.T) {
	h := New(&Config{ServiceName: serviceName, BasePath: servicePath})

	container := restful.NewContainer()
	for _, webService := range h.AddWebservice() {
		container = container.Add(webService)
	}

	h.AddHealthCheck("test", testURL, func() error { return nil })

	resp, _, err :=
		caller.Call(container).
			To(gorequest.New().
				Get("/healthz").
				MakeRequest()).
			Read(&response{}).
			Execute()
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.Code)

	resp2, _, err2 :=
		caller.Call(container).
			To(gorequest.New().
				Get("/" + servicePath + "/healthz").
				MakeRequest()).
			Read(&response{}).
			Execute()
	require.NoError(t, err2, err2)
	assert.Equal(t, http.StatusOK, resp2.Code)
}

func Test_endpointV1(t *testing.T) {
	h := New(&Config{ServiceName: serviceName, BasePath: servicePath})
	container := restfulV1.NewContainer()

	for _, webService := range h.AddWebserviceV1() {
		container.Add(webService)
	}

	h.AddHealthCheck("test", testURL, func() error { return nil })

	resp, _, err :=
		caller.Call(container).
			To(gorequest.New().
				Get("/healthz").
				MakeRequest()).
			Read(&response{}).
			Execute()
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.Code)

	resp2, _, err2 :=
		caller.Call(container).
			To(gorequest.New().
				Get("/" + servicePath + "/healthz").
				MakeRequest()).
			Read(&response{}).
			Execute()
	require.NoError(t, err2, err2)
	assert.Equal(t, http.StatusOK, resp2.Code)
}

// nolint: funlen
func Test_AddHealthCheck(t *testing.T) {
	type arg struct {
		name             string
		checkFunc        CheckFunc
		isHardDependency bool
		healthy          bool
	}

	tests := []struct {
		name string
		args []arg
		want bool
	}{
		{
			name: "test healthy response",
			args: []arg{
				{
					name:             "test1",
					checkFunc:        func() error { return nil },
					isHardDependency: false,
					healthy:          true,
				},
				{
					name:             "test2",
					checkFunc:        func() error { return nil },
					isHardDependency: true,
					healthy:          true,
				},
			},
			want: true,
		},
		{
			name: "test unhealthy response",
			args: []arg{
				{
					name:             "test1",
					checkFunc:        func() error { return fmt.Errorf("error1") },
					isHardDependency: true,
					healthy:          false,
				},
				{
					name:             "test2",
					checkFunc:        func() error { return fmt.Errorf("error2") },
					isHardDependency: true,
					healthy:          false,
				},
			},
			want: false,
		},
		{
			name: "test unhealthy response different hard dependency healthy result",
			args: []arg{
				{
					name:             "test1",
					checkFunc:        func() error { return fmt.Errorf("error1") },
					isHardDependency: true,
					healthy:          false,
				},
				{
					name:             "test2",
					checkFunc:        func() error { return nil },
					isHardDependency: true,
					healthy:          true,
				},
			},
			want: false,
		},
		{
			name: "test healthy response with an unhealthy soft dependency",
			args: []arg{
				{
					name:             "test1",
					checkFunc:        func() error { return fmt.Errorf("error1") },
					isHardDependency: false,
					healthy:          false,
				},
				{
					name:             "test2",
					checkFunc:        func() error { return nil },
					isHardDependency: false,
					healthy:          true,
				},
			},
			want: true,
		},
		{
			name: "test healthy response with unhealthy soft dependencies",
			args: []arg{
				{
					name:             "test1",
					checkFunc:        func() error { return fmt.Errorf("error1") },
					isHardDependency: false,
					healthy:          false,
				},
				{
					name:             "test2",
					checkFunc:        func() error { return fmt.Errorf("error2") },
					isHardDependency: false,
					healthy:          false,
				},
			},
			want: true,
		},
		{
			name: "test unhealthy response with unhealthy soft dependency and unhealthy hard dependency",
			args: []arg{
				{
					name:             "test1",
					checkFunc:        func() error { return fmt.Errorf("error1") },
					isHardDependency: false,
					healthy:          false,
				},
				{
					name:             "test2",
					checkFunc:        func() error { return fmt.Errorf("error2") },
					isHardDependency: true,
					healthy:          false,
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			h := New(&Config{ServiceName: serviceName, BasePath: servicePath})
			expected := response{Name: serviceName, Healthy: true}

			for _, arg := range tt.args {
				if arg.isHardDependency {
					h.AddHardHealthCheck(arg.name, testURL, arg.checkFunc)
				} else {
					h.AddHealthCheck(arg.name, testURL, arg.checkFunc)
				}
				expected.Dependencies = append(expected.Dependencies,
					healthDependency{
						Name:    arg.name,
						URL:     testURL,
						Healthy: arg.healthy,
					})
			}

			h.StartBackgroundCheck(context.Background())
			time.Sleep(2 * time.Second)

			container := restful.NewContainer()
			for _, webService := range h.AddWebservice() {
				container.Add(webService)
			}

			resp, _, err :=
				caller.Call(container).
					To(gorequest.New().
						Get("/healthz").
						MakeRequest()).
					Read(&response{}).
					Execute()
			require.NoError(t, err)

			if tt.want {
				require.Equal(t, http.StatusOK, resp.Code)
			} else {
				require.Equal(t, http.StatusServiceUnavailable, resp.Code)
			}

			var responseV3 response
			_ = json.Unmarshal(resp.Body.Bytes(), &responseV3)
			require.Equal(t, tt.want, responseV3.Healthy)
			require.Equal(t, len(tt.args), len(responseV3.Dependencies))

			containerV1 := restfulV1.NewContainer()
			for _, webService := range h.AddWebserviceV1() {
				containerV1.Add(webService)
			}

			resp, _, err =
				caller.Call(containerV1).
					To(gorequest.New().
						Get("/healthz").
						MakeRequest()).
					Read(&response{}).
					Execute()
			require.NoError(t, err)

			if tt.want {
				require.Equal(t, http.StatusOK, resp.Code)
			} else {
				require.Equal(t, http.StatusServiceUnavailable, resp.Code)
			}

			var responseV1 response
			_ = json.Unmarshal(resp.Body.Bytes(), &responseV1)
			require.Equal(t, tt.want, responseV1.Healthy)
			require.Equal(t, len(tt.args), len(responseV1.Dependencies))
		})
	}
}
