# ConfigCat SDK for Go
https://configcat.com

ConfigCat SDK for Go provides easy integration for your application to ConfigCat.

ConfigCat is a feature flag and configuration management service that lets you separate releases from deployments. You can turn your features ON/OFF using <a href="https://app.configcat.com" target="_blank">ConfigCat Dashboard</a> even after they are deployed. ConfigCat lets you target specific groups of users based on region, email or any other custom user attribute.

ConfigCat is a <a target="_blank" href="https://configcat.com">hosted feature flag service</a>. Manage feature toggles across frontend, backend, mobile, desktop apps. <a target="_blank" href="https://configcat.com">Alternative to LaunchDarkly</a>. Management app + feature flag SDKs.

[![Build Status](https://github.com/configcat/go-sdk/actions/workflows/go-ci.yml/badge.svg?branch=v9)](https://github.com/configcat/go-sdk/actions/workflows/go-ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/configcat/go-sdk/v9)](https://goreportcard.com/report/github.com/configcat/go-sdk/v9)
[![codecov](https://codecov.io/gh/configcat/go-sdk/branch/v9/graph/badge.svg)](https://codecov.io/gh/configcat/go-sdk)
[![GoDoc](https://godoc.org/github.com/configcat/go-sdk?status.svg)](https://pkg.go.dev/github.com/configcat/go-sdk/v9)
![License](https://img.shields.io/github/license/configcat/go-sdk.svg)

## Getting started

### 1. Install the package with `go`
```bash
go get github.com/configcat/go-sdk/v9
```

### 2. Go to the <a href="https://app.configcat.com/sdkkey" target="_blank">ConfigCat Dashboard</a> to get your *SDK Key*:
![SDK-KEY](https://raw.githubusercontent.com/ConfigCat/go-sdk/master/media/readme02-3.png  "SDK-KEY")


### 3. Import the *ConfigCat* client package to your application
```go
import "github.com/configcat/go-sdk/v9"
```

### 4. Create a *ConfigCat* client instance:
```go
client := configcat.NewClient("#YOUR-SDK-KEY#")
```

### 5. Get your setting value:
```go
isMyAwesomeFeatureEnabled := client.GetBoolValue("isMyAwesomeFeatureEnabled", false, nil)
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

Using this feature, you will be able to get different setting values for different users in your application by passing a `UserData` struct to the specific setting evaluation method (`GetBoolValue()`, `GetStringValue()`, `GetIntValue()`, `GetFloatValue()`).

Read more about [Targeting here](https://configcat.com/docs/advanced/targeting/).
```go
user := &configcat.UserData{Identifier: "#USER-IDENTIFIER#"}

isMyAwesomeFeatureEnabled := client.GetBoolValue("isMyAwesomeFeatureEnabled", false, user)
if isMyAwesomeFeatureEnabled {
    DoTheNewThing()
} else {
    DoTheOldThing()
}
```

## Polling Modes
The ConfigCat SDK supports 3 different polling mechanisms to acquire the setting values from ConfigCat. After latest setting values are downloaded, they are stored in the internal cache then all requests are served from there. Read more about Polling Modes and how to use them at [ConfigCat Docs](https://configcat.com/docs/sdk-reference/go/).

## Need help?
https://configcat.com/support

## Contributing
Contributions are welcome. For more info please read the [Contribution Guideline](CONTRIBUTING.md).

## About ConfigCat
- [Official ConfigCat SDKs for other platforms](https://github.com/configcat)
- [Documentation](https://configcat.com/docs)
- [Blog](https://configcat.com/blog)
