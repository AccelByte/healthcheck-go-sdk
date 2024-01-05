# healthcheck-go-sdk

This is library for implementing k8s health check with [go-restful](https://github.com/emicklei/go-restful)

It provides dependencies health checking which returns dependency info, health status, timestamp and last error information
of the dependency check result.


## What's new in v2
- Support soft and hard dependency option. `AddHealthCheck` defaults to soft and `AddHardHealthCheck` is available for
  registering a hard dependency. A soft dependency will not affect the overall healthy status of the service if the dependency is not healthy,
  meanwhile a hard dependency health will.
- Support background periodic check instead of immediate check on every `/healthz` request
- The `/healthz` response now returns last call time, last known good call time, and last error message of the corresponding dependency health check
- New `UpdateHealth` method to enable the implementer service to update a dependency health
- CheckFunc templates improvements

## Usage
#### Installing
```
go get -u github.com/AccelByte/healthcheck-go-sdk/v2
```
**NOTE:** since the v2 includes [eventstream-go-sdk v4](https://github.com/AccelByte/eventstream-go-sdk) template check function, you will need to make sure that cgo is enabled (configurable with `CGO_ENABLED=1` environment variable) and parsing `-tags musl` param when building your Go application in Alpine Linux.

Reference: https://github.com/AccelByte/eventstream-go-sdk#v4



#### Initiating
```go
h := healthcheck.New(&healthcheck.Config{
ServiceName: "serviceName",
BasePath: "/servicePath"},
BackgroundCheckInterval: 60*time.Second,
})
```

#### Registering a (soft) dependency (recommended)
```go
redisClient := new(redis.Client)
timeout := 5 * time.Second
h.AddHealthCheck("redis", "redis:6379", h.RedisHealthCheck(redisClient, timeout))
```

#### Registering a hard dependency
```go
h.AddHardHealthCheck("other-dependency", "dependency:1234", func() error {
// do checking
return nil
})
```

#### Use periodic background checking (recommended)
```go
h.StartBackgroundCheck(ctx)
````

#### Registering health check webservice to a go-restful container
```go
serviceContainer := restful.NewContainer()
...
serviceContainer.Add(h.AddWebservice())
```


### Methods for Updating Health Dependency

There are two ways for a dependency health to be updated. 

The first one is by attaching a check function when adding a dependency using `AddHealthCheck` or `AddHardHealthCheck` 
as shown in the previous section. It suits well if the dependency already provides a way for doing health check, 
hence the check can be done easily inside the check function.

When a dependency does not provide a straight-forward way for health check and there is no easy workaround, we can use 
`UpdateHealth` instead to update the  dependency health. The idea is to update the dependency health status right after 
the dependency is called.

```go
h.AddHealthCheck("emailProvider", "https://email-provider", nil) // register dependency health check with nil check function

...

err := emailProvider.Send(from, to, emailBody)
if err != nil {
	_ = h.UpdateHealth("emailProvider", false, &healthcheck.CheckError{Timestamp: time.Now(), Message: err.Error()}) // update health to false with check error included
	return err
}

_ = h.UpdateHealth("emailProvider", true, nil) // update to healthy if succeed
```



### Check Funtion Templates
Health check function templates are available at [checks.go](checks.go)


