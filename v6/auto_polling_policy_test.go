package configcat

import (
	"testing"
	"time"
)

func TestAutoPollingPolicy_GetConfigurationAsync(t *testing.T) {
	fetcher := newFakeConfigProvider()

	fetcher.SetResponse(fetchResponse{status: Fetched, body: "test"})
	logger := DefaultLogger(LogLevelWarn)
	policy := newAutoPollingPolicy(
		fetcher,
		newInMemoryConfigCache(),
		logger,
		"",
		autoPollConfig{time.Second * 2, nil},
	)
	defer policy.close()

	config := policy.getConfigurationAsync().get().(string)

	if config != "test" {
		t.Error("Expecting test as result")
	}

	fetcher.SetResponse(fetchResponse{status: Fetched, body: "test2"})
	config = policy.getConfigurationAsync().get().(string)

	if config != "test" {
		t.Error("Expecting test as result")
	}

	time.Sleep(time.Second * 4)
	config = policy.getConfigurationAsync().get().(string)

	if config != "test2" {
		t.Error("Expecting test2 as result")
	}
}

func TestAutoPollingPolicy_GetConfigurationAsync_Fail(t *testing.T) {
	fetcher := newFakeConfigProvider()

	fetcher.SetResponse(fetchResponse{status: Failure, body: ""})
	logger := DefaultLogger(LogLevelWarn)
	policy := newAutoPollingPolicy(
		fetcher,
		newInMemoryConfigCache(),
		logger,
		"",
		autoPollConfig{time.Second * 2, nil},
	)
	defer policy.close()

	config := policy.getConfigurationAsync().get().(string)

	if config != "" {
		t.Error("Expecting default")
	}
}

func TestAutoPollingPolicy_GetConfigurationAsync_WithListener(t *testing.T) {
	fetcher := newFakeConfigProvider()
	logger := DefaultLogger(LogLevelWarn)
	fetcher.SetResponse(fetchResponse{status: Fetched, body: "test"})
	c := make(chan bool, 1)
	defer close(c)
	policy := newAutoPollingPolicy(
		fetcher,
		newInMemoryConfigCache(),
		logger,
		"",
		AutoPollWithChangeListener(
			time.Second*2,
			func() { c <- true },
		).(autoPollConfig),
	)
	defer policy.close()
	called := <-c

	if !called {
		t.Error("Expecting test as result")
	}
}
