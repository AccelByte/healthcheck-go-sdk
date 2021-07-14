# healthcheck-go-sdk

This is library for implementing k8s health check with [go-restful](https://github.com/emicklei/go-restful)

## Example
```go
h := healthcheck.New("service-name", "serviceBasePath")

// redis health check example
redisClient := new(redis.Client)
timeout := 5 * time.Second
h.AddHealthCheck("redis", "redis:6379", h.RedisHealthCheck(redisClient, timeout))

container := restful.NewContainer().Add(h.AddWebservice())

http.ListenAndServe(":8000", container)
```

Other health check function available at [checks.go](checks.go)


