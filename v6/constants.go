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
	// Global indicates that your data will be published to all ConfigCat CDN nodes to guarantee lowest response times.
	Global DataGovernance = 0
	// EuOnly indicates that your data will be published to CDN nodes only in the EU.
	EuOnly DataGovernance = 1
)

const (
	globalBaseUrl = "https://cdn-global.configcat.com"
	euOnlyBaseUrl = "https://cdn-eu.configcat.com"
)

const (
	no  = 0
	yes = 1
)

// async statuses
const (
	pending   = 0
	completed = 1
)

const (
	entries     = "f"
	preferences = "p"

	preferencesUrl      = "u"
	preferencesRedirect = "r"

	settingValue                  = "v"
	settingType                   = "t"
	settingRolloutPercentageItems = "p"
	settingRolloutRules           = "r"
	settingVariationId            = "i"

	rolloutValue               = "v"
	rolloutComparisonAttribute = "a"
	rolloutComparator          = "t"
	rolloutComparisonValue     = "c"
	rolloutVariationId         = "i"

	percentageItemValue       = "v"
	percentageItemPercentage  = "p"
	percentageItemVariationId = "i"
)
