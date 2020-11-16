package configcat

// fetchResponse represents a configuration fetch response.
type fetchResponse struct {
	status fetchStatus
	config *config
}

// isFailed returns true if the fetch is failed, otherwise false.
func (response fetchResponse) isFailed() bool {
	return response.status == Failure
}

// isNotModified returns true if if the fetch resulted a 304 Not Modified code, otherwise false.
func (response fetchResponse) isNotModified() bool {
	return response.status == NotModified
}

// isFetched returns true if a new configuration value was fetched, otherwise false.
func (response fetchResponse) isFetched() bool {
	return response.status == Fetched
}
