package configcat

// Describes the fetch response statuses.
type FetchStatus int

const (
	// Indicates that a new configuration was fetched.
	Fetched      FetchStatus = 0
	// Indicates that the current configuration is not modified.
	NotModified  FetchStatus = 1
	// Indicates that the current configuration fetch is failed.
	Failure   	 FetchStatus = 2
)

const (
	no   = 0
	yes  = 1
)

// async statuses
const (
	pending     = 0
	completed  	= 1
	cancelled   = 2
)
