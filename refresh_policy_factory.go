package configcat

type pollingModeVisitor interface {
	visitAutoPoll(config autoPollConfig) refreshPolicy
	visitManualPoll(config manualPollConfig) refreshPolicy
	visitLazyLoad(config lazyLoadConfig) refreshPolicy
}

type refreshPolicyFactory struct {
	configFetcher configProvider
	store *configStore
	logger Logger
}

func newRefreshPolicyFactory(configFetcher configProvider, store *configStore, logger Logger) *refreshPolicyFactory {
	return &refreshPolicyFactory{ configFetcher: configFetcher, store: store, logger:logger }
}

func (factory *refreshPolicyFactory) visitAutoPoll(config autoPollConfig) refreshPolicy {
	return newAutoPollingPolicy(factory.configFetcher, factory.store, factory.logger, config)
}

func (factory *refreshPolicyFactory) visitManualPoll(config manualPollConfig) refreshPolicy {
	return newManualPollingPolicy(factory.configFetcher, factory.store, factory.logger)
}

func (factory *refreshPolicyFactory) visitLazyLoad(config lazyLoadConfig) refreshPolicy {
	return newLazyLoadingPolicy(factory.configFetcher, factory.store, factory.logger, config)
}

