package configcat

import (
	"context"
	"time"
)

type manualPollConfig struct{}

func (config manualPollConfig) getModeIdentifier() string {
	return "m"
}

func (config manualPollConfig) refreshPolicy(rconfig refreshPolicyConfig) refreshPolicy {
	return newManualPollingPolicy(rconfig)
}

// ManualPoll creates a manual loading refresh mode which fetches the latest configuration
// only when explicitly refreshed. It uses the cache if a configuration hasn't been fetched.
func ManualPoll() RefreshMode {
	return manualPollConfig{}
}

type manualPollingPolicy struct {
	fetcher *configFetcher
}

// newManualPollingPolicy initializes a new manualPollingPolicy.
func newManualPollingPolicy(rconfig refreshPolicyConfig) *manualPollingPolicy {
	return &manualPollingPolicy{
		fetcher: rconfig.fetcher,
	}
}

var expiredContext, _ = context.WithDeadline(context.Background(), time.Time{})

func (p *manualPollingPolicy) get(ctx context.Context) *config {
	// Note: use an expired context so we'll always hit the cache
	// if there's no configuration immediately available.
	return p.fetcher.config(expiredContext)
}

func (p *manualPollingPolicy) refresh(ctx context.Context) {
	p.fetcher.refresh(ctx)
}

func (policy *manualPollingPolicy) close() {
}
