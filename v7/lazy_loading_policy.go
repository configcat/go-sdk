package configcat

import (
	"context"
	"time"
)

// lazyLoadConfig describes the configuration for auto polling.
type lazyLoadConfig struct {
	// The cache invalidation interval.
	cacheInterval time.Duration
	// If you use the asynchronous refresh then when a request is being made on the cache while it's expired,
	// the previous value will be returned immediately until the fetching of the new configuration is completed
	useAsyncRefresh bool
}

func (config lazyLoadConfig) getModeIdentifier() string {
	return "l"
}

func (config lazyLoadConfig) refreshPolicy(rconfig refreshPolicyConfig) refreshPolicy {
	if config.useAsyncRefresh {
		return newLazyLoadingPolicyWithAsyncRefresh(config, rconfig)
	}
	return newLazyLoadingPolicyWithSyncRefresh(config, rconfig)
}

// LazyLoad creates a lazy loading refresh mode. The configuration is fetched
// on demand the first time it's needed and then when the
// previous fetch is more than cacheInterval earlier.
// If useAsyncRefresh is true, the previous configuration will continue
// to be used while the configuration is refreshing rather
// than waiting for the new configuration to be received.
func LazyLoad(cacheInterval time.Duration, useAsyncRefresh bool) RefreshMode {
	return lazyLoadConfig{
		cacheInterval:   cacheInterval,
		useAsyncRefresh: useAsyncRefresh,
	}
}

type lazyLoadingPolicyWithAsyncRefresh struct {
	cacheInterval time.Duration
	fetcher       *configFetcher
}

func newLazyLoadingPolicyWithAsyncRefresh(
	config lazyLoadConfig,
	rconfig refreshPolicyConfig,
) *lazyLoadingPolicyWithAsyncRefresh {
	return &lazyLoadingPolicyWithAsyncRefresh{
		fetcher:       rconfig.fetcher,
		cacheInterval: config.cacheInterval,
	}
}

func (p *lazyLoadingPolicyWithAsyncRefresh) get(ctx context.Context) *config {
	if conf := p.fetcher.config(ctx); conf != nil {
		if time.Since(conf.fetchTime) >= p.cacheInterval {
			p.fetcher.startRefresh()
		}
		return conf
	}
	p.fetcher.startRefresh()
	return p.fetcher.config(ctx)
}

func (p *lazyLoadingPolicyWithAsyncRefresh) refresh(ctx context.Context) {
	p.fetcher.refresh(ctx)
}

func (p *lazyLoadingPolicyWithAsyncRefresh) close() {
}

type lazyLoadingPolicyWithSyncRefresh struct {
	cacheInterval time.Duration
	fetcher       *configFetcher
}

func newLazyLoadingPolicyWithSyncRefresh(
	config lazyLoadConfig,
	rconfig refreshPolicyConfig,
) *lazyLoadingPolicyWithSyncRefresh {
	return &lazyLoadingPolicyWithSyncRefresh{
		fetcher:       rconfig.fetcher,
		cacheInterval: config.cacheInterval,
	}
}

func (p *lazyLoadingPolicyWithSyncRefresh) get(ctx context.Context) *config {
	if conf := p.fetcher.config(ctx); conf != nil && time.Since(conf.fetchTime) < p.cacheInterval {
		return conf
	}
	p.fetcher.invalidateAndStartRefresh()
	return p.fetcher.config(ctx)
}

func (p *lazyLoadingPolicyWithSyncRefresh) refresh(ctx context.Context) {
	p.fetcher.refresh(ctx)
}

func (p *lazyLoadingPolicyWithSyncRefresh) close() {
}
