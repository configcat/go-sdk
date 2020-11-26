# ConfigCat SDK for Go
https://configcat.com

ConfigCat SDK for Go provides easy integration for your application to ConfigCat.

ConfigCat is a feature flag and configuration management service that lets you separate releases from deployments. You can turn your features ON/OFF using <a href="https://app.configcat.com" target="_blank">ConfigCat Dashboard</a> even after they are deployed. ConfigCat lets you target specific groups of users based on region, email or any other custom user attribute.

ConfigCat is a <a target="_blank" href="https://configcat.com">hosted feature flag service</a>. Manage feature toggles across frontend, backend, mobile, desktop apps. <a target="_blank" href="https://configcat.com">Alternative to LaunchDarkly</a>. Management app + feature flag SDKs.

[![Build Status](https://travis-ci.com/configcat/go-sdk.svg?branch=master)](https://travis-ci.com/configcat/go-sdk)
[![Go Report Card](https://goreportcard.com/badge/github.com/configcat/go-sdk)](https://goreportcard.com/report/github.com/configcat/go-sdk)
[![codecov](https://codecov.io/gh/configcat/go-sdk/branch/master/graph/badge.svg)](https://codecov.io/gh/configcat/go-sdk)
[![GoDoc](https://godoc.org/github.com/configcat/go-sdk?status.svg)](https://pkg.go.dev/github.com/configcat/go-sdk/v5)
![License](https://img.shields.io/github/license/configcat/go-sdk.svg)

## Getting started

### 1. Install the package with `go`
```bash
go get github.com/configcat/go-sdk/v6
```

### 2. Go to <a href="https://app.configcat.com/sdkkey" target="_blank">Connect your application</a> tab to get your *SDK Key*:
![SDK-KEY](https://raw.githubusercontent.com/ConfigCat/go-sdk/master/media/readme01.png  "SDK-KEY")


### 3. Import the *ConfigCat* client package to your application
```go
import "github.com/configcat/go-sdk/v7"
```

### 4. Create a *ConfigCat* client instance:
```go
client := configcat.NewClient("#YOUR-SDK-KEY#")
```

### 5. Get your setting value:
```go
isMyAwesomeFeatureEnabled := client.Bool("isMyAwesomeFeatureEnabled", false, nil)
if isMyAwesomeFeatureEnabled {
    DoTheNewThing()
} else {
    DoTheOldThing()
}
```

### 6. Close *ConfigCat* client on application exit:
```go
client.Close()
```


## Getting user specific setting values with Targeting

Using this feature, you will be able to get different setting values for different users in your application by passing a `User` value to the `ValueForUser()` method.

You can use the `UserValue` struct for this, or implement your own struct type. The client will inspect values in struct fields; see the documentation for more details.

Read more about [Targeting here](https://configcat.com/docs/advanced/targeting/).
```go
user := configcat.NewUser("#USER-IDENTIFIER#")

isMyAwesomeFeatureEnabled, ok := client.Bool("isMyAwesomeFeatureEnabled", false, user)
if isMyAwesomeFeatureEnabled {
    DoTheNewThing()
} else {
    DoTheOldThing()
}
```

## Polling
By default, the ConfigCat client will poll to find out when the values have changed. The polling interval can be configured in the `Config` that's passed to `NewCustomClient`. It's also possible to manually poll using the `Client.Refresh` method.

## Need help?
https://configcat.com/support

## Contributing
Contributions are welcome.

## About ConfigCat
- [Official ConfigCat SDKs for other platforms](https://github.com/configcat)
- [Documentation](https://configcat.com/docs)
- [Blog](https://configcat.com/blog)
