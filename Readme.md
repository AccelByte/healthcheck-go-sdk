# healthcheck-go-sdk

> [!WARNING]
> **Deprecated — no longer maintained.** Both `v1` and `v2` of this module are frozen:
> no new features, bug fixes or security patches will be released. Existing versions
> remain available to read and `go get`, but do not start new work on this module.
>
> Do not add this module as a dependency of new projects.

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


