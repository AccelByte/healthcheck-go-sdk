package healthcheck

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	caller "github.com/AccelByte/http-test-caller"
	"github.com/emicklei/go-restful/v3"
	"github.com/parnurzeal/gorequest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testURL     = "www.test.example.com"
	serviceName = "test"
	servicePath = "service"
)

func Test_endpoint(t *testing.T) {
	h := New(serviceName, servicePath)
	container := restful.NewContainer().Add(h.AddWebservice())
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

func Test_AddHealthCheck(t *testing.T) {
	type arg struct {
		name      string
		checkFunc CheckFunc
		healthy   bool
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
					name:      "test1",
					checkFunc: func() error { return nil },
					healthy:   true,
				},
				{
					name:      "test2",
					checkFunc: func() error { return nil },
					healthy:   true,
				},
			},
			want: true,
		},
		{
			name: "test unhealthy response",
			args: []arg{
				{
					name:      "test1",
					checkFunc: func() error { return fmt.Errorf("error1") },
					healthy:   false,
				},
				{
					name:      "test2",
					checkFunc: func() error { return fmt.Errorf("error2") },
					healthy:   false,
				},
			},
			want: false,
		},
		{
			name: "test unhealthy response different healthy result",
			args: []arg{
				{
					name:      "test1",
					checkFunc: func() error { return fmt.Errorf("error1") },
					healthy:   false,
				},
				{
					name:      "test2",
					checkFunc: func() error { return nil },
					healthy:   true,
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			h := New(serviceName, servicePath)
			expected := response{Name: serviceName, Healthy: true}

			for _, arg := range tt.args {
				h.AddHealthCheck(arg.name, testURL, arg.checkFunc)
				expected.Dependencies = append(expected.Dependencies,
					healthDependency{
						Name:    arg.name,
						URL:     testURL,
						Healthy: arg.healthy,
					})
			}

			container := restful.NewContainer().Add(h.AddWebservice())
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

			var response response
			_ = json.Unmarshal(resp.Body.Bytes(), &response)
			require.Equal(t, tt.want, response.Healthy)
			require.Equal(t, len(tt.args), len(response.Dependencies))
		})
	}
}
