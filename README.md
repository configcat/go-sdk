# ConfigCat SDK for Go

ConfigCat SDK for Go provides easy integration between ConfigCat service and applications using Go.

ConfigCat is a feature flag, feature toggle, and configuration management service. That lets you launch new features and change your software configuration remotely without actually (re)deploying code. ConfigCat even helps you do controlled roll-outs like canary releases and blue-green deployments.
https://configcat.com  

[![Build Status](https://travis-ci.org/configcat/go-sdk.svg?branch=master)](https://travis-ci.org/configcat/go-sdk)
[![Go Report Card](https://goreportcard.com/badge/github.com/configcat/go-sdk)](https://goreportcard.com/report/github.com/configcat/go-sdk)
[![codecov](https://codecov.io/gh/configcat/go-sdk/branch/master/graph/badge.svg)](https://codecov.io/gh/configcat/go-sdk)
[![GoDoc](https://godoc.org/github.com/configcat/go-sdk?status.svg)](https://godoc.org/github.com/configcat/go-sdk)
![License](https://img.shields.io/github/license/configcat/go-sdk.svg)

## Getting started

### 1. Install the package with `go`
```bash
go get gopkg.in/configcat/go-sdk.v1
```

### 2. <a href="https://configcat.com/Account/Login" target="_blank">Log in to ConfigCat Management Console</a> and go to your *Project* to get your *API Key*:
![API-KEY](https://raw.githubusercontent.com/ConfigCat/go-sdk/master/media/readme01.png  "API-KEY")


### 3. Import the *ConfigCat* client package to your application
```go
import gopkg.in/configcat/go-sdk.v1
```

### 4. Create a *ConfigCat* client instance:
```go
client := configcat.NewClient("#YOUR-API-KEY#")
```

### 5. Get your setting value:
```go
isMyAwesomeFeatureEnabled, ok := client.GetValue("isMyAwesomeFeatureEnabled", false).(bool)
if ok && isMyAwesomeFeatureEnabled {
    DoTheNewThing()
} else {
    DoTheOldThing()
}
```
Or use the async APIs:
```go
client.GetValueAsync("isMyAwesomeFeatureEnabled", false, func(result interface{}) {
    isMyAwesomeFeatureEnabled, ok := result.(bool)
    if ok && isMyAwesomeFeatureEnabled {
        DoTheNewThing()
    } else {
        DoTheOldThing()
    }
})
```

### 6. Close *ConfigCat* client on application exit:
```go
client.Close()
```


## Getting user specific setting values with Targeting
Using this feature, you will be able to get different setting values for different users in your application by passing a `User Object` to the `getValue()` function.

Read more about [Targeting here](https://docs.configcat.com/docs/advanced/targeting/).
```go
user := configcat.NewUser("#USER-IDENTIFIER#")

isMyAwesomeFeatureEnabled, ok := client.GetValueForUser("isMyAwesomeFeatureEnabled", user, false).(bool)
if ok && isMyAwesomeFeatureEnabled {
    DoTheNewThing()
} else {
    DoTheOldThing()
}
```

## Polling Modes
The ConfigCat SDK supports 3 different polling mechanisms to acquire the setting values from ConfigCat. After latest setting values are downloaded, they are stored in the internal cache then all requests are served from there. Read more about Polling Modes and how to use them at [ConfigCat Docs](https://docs.configcat.com/docs/sdk-reference/go/).

## Support
If you need help how to use this SDK feel free to to contact the ConfigCat Staff on https://configcat.com. We're happy to help.

## Contributing
Contributions are welcome.

## License
[MIT](https://raw.githubusercontent.com/ConfigCat/go-sdk/master/LICENSE)
