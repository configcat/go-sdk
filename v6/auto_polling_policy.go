package configcat

import (
	"context"
	"sync"
	"time"
)

// autoPollConfig describes the configuration for auto polling.
type autoPollConfig struct {
	interval     time.Duration
	changeNotify func()
}

func (config autoPollConfig) getModeIdentifier() string {
	return "a"
}

func (config autoPollConfig) refreshPolicy(rconfig refreshPolicyConfig) refreshPolicy {
	return newAutoPollingPolicy(config, rconfig)
}

// AutoPoll creates an auto polling refresh mode that polls for changes
// at the given interval.
func AutoPoll(interval time.Duration) RefreshMode {
	return autoPollConfig{
		interval: interval,
	}
}

// AutoPollWithChangeListener creates an auto polling refresh mode. It polls for changes
// at the given interval, and calls changeNotify whenever the configuration has changed.
//
// If changeNotify is nil, this is equvalent to AutoPoll.
func AutoPollWithChangeListener(
	interval time.Duration,
	changeNotify func(),
) RefreshMode {
	return autoPollConfig{
		interval:     interval,
		changeNotify: changeNotify,
	}
}

// autoPollingPolicy implements the AutoPoll policy.
type autoPollingPolicy struct {
	fetcher      *configFetcher
	changeNotify func()
	logger       Logger
	// mu guards the closing of the closed channel.
	mu     sync.Mutex
	closed chan struct{}
}

// newAutoPollingPolicy initializes a new autoPollingPolicy.
func newAutoPollingPolicy(
	config autoPollConfig,
	rconfig refreshPolicyConfig,
) *autoPollingPolicy {
	policy := &autoPollingPolicy{
		fetcher:      rconfig.fetcher,
		closed:       make(chan struct{}),
		changeNotify: config.changeNotify,
		logger:       rconfig.logger,
	}
	policy.logger.Debugf("Auto polling started with %+v interval.", config.interval)
	policy.fetcher.startRefresh()
	go policy.poller(config.interval)
	return policy
}

func (p *autoPollingPolicy) refresh(ctx context.Context) {
	p.fetcher.refresh(ctx)
}

func (p *autoPollingPolicy) get(ctx context.Context) *config {
	return p.fetcher.config(ctx)
}

func (policy *autoPollingPolicy) poller(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-policy.closed
		cancel()
	}()
	config := policy.fetcher.config(ctx)
	policy.notify()
	for {
		select {
		case <-policy.closed:
			policy.logger.Debugf("Auto polling stopped.")
			return
		case <-ticker.C:
			policy.refresh(ctx)
			newConfig := policy.fetcher.config(ctx)
			if newConfig.body() != config.body() {
				config = newConfig
				policy.notify()
			}
		}
	}
}

// notify calls the changeNotify function if necessary.
func (policy *autoPollingPolicy) notify() {
	if policy.changeNotify != nil {
		go policy.changeNotify()
	}
}

func (policy *autoPollingPolicy) close() {
	// Guard against double-close.
	policy.mu.Lock()
	defer policy.mu.Unlock()
	select {
	case <-policy.closed:
		return
	default:
		close(policy.closed)
	}
}
