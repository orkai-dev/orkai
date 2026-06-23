package providers

import (
	"log/slog"
	"net/http"
)

type factoryDeps struct {
	settings SettingsGetter
	client   *http.Client
	logger   *slog.Logger
}

var (
	gitFactoryFns        []func(factoryDeps) GitProvider
	regFactoryFns        []func(factoryDeps) RegistryProvider
	objStorageFactoryFns []func(factoryDeps) ObjectStorageProvider
)

func registerGit(factory func(factoryDeps) GitProvider) {
	gitFactoryFns = append(gitFactoryFns, factory)
}

func registerRegistry(factory func(factoryDeps) RegistryProvider) {
	regFactoryFns = append(regFactoryFns, factory)
}

func registerObjectStorage(factory func(factoryDeps) ObjectStorageProvider) {
	objStorageFactoryFns = append(objStorageFactoryFns, factory)
}
