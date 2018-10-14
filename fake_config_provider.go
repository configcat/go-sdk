package configcat

import "time"

type fakeConfigProvider struct {
	result        FetchResponse
	sleepDuration time.Duration
}

func newFakeConfigProvider() *fakeConfigProvider {
	return &fakeConfigProvider{}
}

func (fetcher *fakeConfigProvider) GetConfigurationAsync() *AsyncResult {
	async := NewAsyncResult()
	go func() {
		if fetcher.sleepDuration > 0 {
			time.Sleep(fetcher.sleepDuration)
		}
		async.Complete(fetcher.result)
	}()

	return async
}

func (fetcher *fakeConfigProvider) SetResponse(response FetchResponse) {
	fetcher.result = response
}

func (fetcher *fakeConfigProvider) SetResponseWithDelay(response FetchResponse, delayDuration time.Duration) {
	fetcher.sleepDuration = delayDuration
	fetcher.result = response
}
