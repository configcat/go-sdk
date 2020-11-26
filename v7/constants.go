package configcat

// FetchStatus describes the fetch response statuses.
type fetchStatus int

const (
	// Fetched indicates that a new configuration was fetched.
	Fetched fetchStatus = 0
	// NotModified indicates that the current configuration is not modified.
	NotModified fetchStatus = 1
	// Failure indicates that the current configuration fetch is failed.
	Failure fetchStatus = 2
)

// DataGovernance describes the location of your feature flag and setting data within the ConfigCat CDN.
type DataGovernance int

const (
	// Global Select this if your feature flags are published to all global CDN nodes.
	Global DataGovernance = 0
	// EuOnly Select this if your feature flags are published to CDN nodes only in the EU.
	EuOnly DataGovernance = 1
)

const (
	globalBaseUrl = "https://cdn-global.configcat.com"
	euOnlyBaseUrl = "https://cdn-eu.configcat.com"
)
