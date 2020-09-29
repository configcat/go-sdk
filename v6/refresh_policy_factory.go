package configcat

type pollingModeVisitor interface {
	visitAutoPoll(config autoPollConfig) refreshPolicy
	visitManualPoll(config manualPollConfig) refreshPolicy
	visitLazyLoad(config lazyLoadConfig) refreshPolicy
}

type refreshPolicyFactory struct {
	configFetcher configProvider
	cache         ConfigCache
	logger        Logger
	sdkKey		  string
}

func newRefreshPolicyFactory(configFetcher configProvider, cache ConfigCache, logger Logger, sdkKey string) *refreshPolicyFactory {
	return &refreshPolicyFactory{configFetcher: configFetcher, cache: cache, logger: logger, sdkKey: sdkKey}
}

func (factory *refreshPolicyFactory) visitAutoPoll(config autoPollConfig) refreshPolicy {
	return newAutoPollingPolicy(factory.configFetcher, factory.cache, factory.logger, factory.sdkKey, config)
}

func (factory *refreshPolicyFactory) visitManualPoll(config manualPollConfig) refreshPolicy {
	return newManualPollingPolicy(factory.configFetcher, factory.cache, factory.logger, factory.sdkKey)
}

func (factory *refreshPolicyFactory) visitLazyLoad(config lazyLoadConfig) refreshPolicy {
	return newLazyLoadingPolicy(factory.configFetcher, factory.cache, factory.logger, factory.sdkKey, config)
}
