# ConfigCat Go SDK
ConfigCat is a cloud based configuration as a service. It integrates with your apps, backends, websites, 
and other programs, so you can configure them through [this](https://configcat.com) website even after they are deployed.

## Getting started

**1. Get the SDK with `go`**

```bash
go get gopkg.in/configcat/go-sdk.v1
```

**2. Get your Api Key from [ConfigCat.com](https://configcat.com) portal**
![YourConnectionUrl](https://raw.githubusercontent.com/ConfigCat/java-sdk/master/media/readme01.png  "ApiKey")

**3. Import the ConfigCat package**
```go
import gopkg.in/configcat/go-sdk.v1
```
**4. Create a ConfigCatClient instance**
```go
client := configcat.NewClient("<PLACE-YOUR-API-KEY-HERE>")
```
**5. (Optional) Prepare a User object for rollout calculation**
```go
user := configcat.NewUser("<PLACE-YOUR-USER-IDENTIFIER-HERE>")
```
**6. Get your config value**
```go
isMyAwesomeFeatureEnabled, ok := client.GetValueForUser("key-of-my-awesome-feature", false, user).(bool)
if ok && isMyAwesomeFeatureEnabled {
    //show your awesome feature to the world!
}
```
Or use the async APIs:
```go
client.GetValueAsyncForUser("key-of-my-awesome-feature", false, func(result interface{}) {
    isMyAwesomeFeatureEnabled, ok := result.(bool)
    if(ok && isMyAwesomeFeatureEnabled) {
        //show your awesome feature to the world! 
    }
})
```
## User object
Percentage and targeted rollouts are calculated by the user object you can optionally pass to the configuration requests.
The user object must be created with a **mandatory** identifier parameter which should uniquely identify each user:
```go
user := configcat.NewUser("<PLACE-YOUR-USER-IDENTIFIER-HERE>" /* mandatory */)
```
But you can also set other custom attributes if you'd like to calculate the rollout based on them:
```go
custom := map[string]string{}
custom["Subscription"] = "Free"
custom["Role"] = "Knight of Awesomnia"
user := configcat.NewUserWithAdditionalAttributes("<PLACE-YOUR-USER-IDENTIFIER-HERE>", // mandatory
    "simple@but.awesome.com", "Awesomnia", custom)
```
## Configuration
### Refresh policies
The internal caching control and the communication between the client and ConfigCat are managed through a refresh policy. There are 3 predefined implementations built in the library.
#### 1. Auto polling policy (default)
This policy fetches the latest configuration and updates the cache repeatedly. 

```go
config := DefaultClientConfig()
config.PolicyFactory = func(configProvider configcat.ConfigProvider, store *configcat.ConfigStore) configcat.RefreshPolicy {
    return configcat.NewAutoPollingPolicy(configProvider, store, 
    // The auto poll interval
    time.Second * 120)
}
       
client := configcat.NewCustomClient("<PLACE-YOUR-API-KEY-HERE>", config)
```

You have the option to configure the polling interval and an `configChanged` callback that will be notified when a new configuration is fetched. The policy calls the given method only, when the new configuration is differs from the cached one.

```go
config := DefaultClientConfig()
config.PolicyFactory = func(configProvider configcat.ConfigProvider, store *configcat.ConfigStore) configcat.RefreshPolicy {
    return configcat.NewAutoPollingPolicy(configProvider, store, 
    // The auto poll interval
    time.Second * 120,
    // The callback called when the configuration changes
    func(config string, parser *configcat.ConfigParser) { 
        isMyAwesomeFeatureEnabled, ok := parser.Parse(config, "key-of-my-awesome-feature").(bool)
        if(ok && isMyAwesomeFeatureEnabled) {
            //show your awesome feature to the world!
        }
    })
}
       
client := configcat.NewCustomClient("<PLACE-YOUR-API-KEY-HERE>", config)
```

#### 2. Expiring cache policy
This policy uses an expiring cache to maintain the internally stored configuration. 
##### Cache refresh interval 
You can define the refresh rate of the cache in seconds, 
after the initial cached value is set this value will be used to determine how much time must pass before initiating a new configuration fetch request through the `ConfigProvider`.
##### Async / Sync refresh
You can define how do you want to handle the expiration of the cached configuration. If you choose asynchronous refresh then 
when a request is being made on the cache while it's expired, the previous value will be returned immediately 
until the fetching of the new configuration is completed.
```go
config := DefaultClientConfig()
config.PolicyFactory = func(configProvider configcat.ConfigProvider, store *configcat.ConfigStore) configcat.RefreshPolicy {
    return configcat.NewExpiringCachePolicy(configProvider, store, 
	// The cache expiration interval
	time.Second * 120,
	// True for async, false for sync refresh
	true)
}
       
client := configcat.NewCustomClient("<PLACE-YOUR-API-KEY-HERE>", config)
```

#### 3. Manual polling policy
With this policy every new configuration request on the ConfigCatClient will trigger a new fetch over HTTP.
```go
config := configcat.DefaultClientConfig()
config.PolicyFactory = func(configProvider configcat.ConfigProvider, store *configcat.ConfigStore) configcat.RefreshPolicy {
    return configcat.NewManualPollingPolicy(configProvider, store)
}
       
client := configcat.NewCustomClient("<PLACE-YOUR-API-KEY-HERE>", config)
```

#### Custom Policy
You can also implement your custom refresh policy by satisfying the `RefreshPolicy` interface.
```go
type CustomPolicy struct {
    configcat.ConfigRefresher
}

func NewCustomPolicy(fetcher configcat.ConfigProvider, store *configcat.ConfigStore) *NewCustomPolicy {
    return &NewCustomPolicy{ ConfigRefresher: ConfigRefresher{ Fetcher:fetcher, Store:store }}
}

func (policy *NewCustomPolicy) GetConfigurationAsync() *configcat.AsyncResult {
    // this method will be called when the configuration is requested from the ConfigCat client.
    // you can access the config fetcher through the policy.Fetcher and the internal store via policy.Store
}
```
> The `AsyncResult` and the `Async` are internal types used to signal back to the caller about the completion of a given task like [Futures](https://en.wikipedia.org/wiki/Futures_and_promises).

Then you can simply inject your custom policy implementation into the ConfigCat client:
```go
config := configcat.DefaultClientConfig()
config.PolicyFactory = func(configProvider configcat.ConfigProvider, store *configcat.ConfigStore) configcat.RefreshPolicy {
    return NewCustomPolicy(configProvider, store)
}

client := configcat.NewCustomClient("<PLACE-YOUR-API-KEY-HERE>", config)
```

### Custom Cache
You have the option to inject your custom cache implementation into the client. All you have to do is to satisfy the `ConfigCache` interface:
```go
type CustomCache struct {
}

func (cache *CustomCache) Get() (string, error) {
    // here you have to return with the cached value
}

func (cache *CustomCache) Set(value string) error {
    // here you have to store the new value in the cache
}
```
Then use your custom cache implementation:
```go      
config := configcat.DefaultClientConfig()
config.Cache = CustomCache{}

client := configcat.NewCustomClient("<PLACE-YOUR-API-KEY-HERE>", config)
```

### Maximum wait time for synchronous calls
You have the option to set a timeout value for the synchronous methods of the library (`GetValue()`, `GetValueForUse()`, `Refresh()`) which means
when a sync call takes longer than the timeout value, it'll return with the default.
```go      
config := configcat.DefaultClientConfig()
config.MaxWaitTimeForSyncCalls = time.Seconds * 10

client := configcat.NewCustomClient("<PLACE-YOUR-API-KEY-HERE>", config)
```

### Force refresh
Any time you want to refresh the cached configuration with the latest one, you can call the `Refresh()` or `RefreshAsync()` method of the library,
which will initiate a new fetch and will update the local cache.
