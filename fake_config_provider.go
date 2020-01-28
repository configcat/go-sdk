package configcat

import "time"

type fakeConfigProvider struct {
	result        fetchResponse
	sleepDuration time.Duration
}

func newFakeConfigProvider() *fakeConfigProvider {
	return &fakeConfigProvider{}
}

func (fetcher *fakeConfigProvider) getConfigurationAsync() *asyncResult {
	async := newAsyncResult()
	go func() {
		if fetcher.sleepDuration > 0 {
			time.Sleep(fetcher.sleepDuration)
		}
		async.complete(fetcher.result)
	}()

	return async
}

func (fetcher *fakeConfigProvider) SetResponse(response fetchResponse) {
	fetcher.result = response
}

func (fetcher *fakeConfigProvider) SetResponseWithDelay(response fetchResponse, delayDuration time.Duration) {
	fetcher.sleepDuration = delayDuration
	fetcher.result = response
}
