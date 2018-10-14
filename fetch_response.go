package configcat

// FetchResponse represents a configuration fetch response.
type FetchResponse struct {
	Status FetchStatus
	Body   string
}

// IsFailed returns true if the fetch is failed, otherwise false.
func (response FetchResponse) IsFailed() bool {
	return response.Status == Failure
}

// IsNotModified returns true if if the fetch resulted a 304 Not Modified code, otherwise false.
func (response FetchResponse) IsNotModified() bool {
	return response.Status == NotModified
}

// IsFetched returns true if a new configuration value was fetched, otherwise false.
func (response FetchResponse) IsFetched() bool {
	return response.Status == Fetched
}
