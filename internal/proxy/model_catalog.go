package proxy

import (
	"context"
	"strings"
)

type ModelCatalog interface {
	LookupProviderModel(ctx context.Context, provider string, model string) (ProviderModel, bool)
}

type ProviderModel struct {
	Provider       string
	CanonicalModel string
	ProviderModel  string
}

type StaticModelCatalog struct {
	models map[string]ProviderModel
}

func NewStaticModelCatalog(models []ProviderModel) *StaticModelCatalog {
	catalog := &StaticModelCatalog{models: map[string]ProviderModel{}}
	for _, model := range models {
		provider := normalizeProvider(model.Provider)
		model.Provider = provider
		if providerModel := strings.TrimSpace(model.ProviderModel); providerModel != "" {
			catalog.models[modelCatalogKey(provider, providerModel)] = model
		}
	}
	return catalog
}

func (c *StaticModelCatalog) LookupProviderModel(_ context.Context, provider string, model string) (ProviderModel, bool) {
	if c == nil {
		return ProviderModel{}, false
	}
	item, ok := c.models[modelCatalogKey(normalizeProvider(provider), strings.TrimSpace(model))]
	return item, ok
}

func modelCatalogKey(provider string, model string) string {
	return provider + "\x00" + model
}

func normalizeProvider(provider string) string {
	provider = strings.TrimSpace(provider)
	if provider == "" {
		return "ark"
	}
	return provider
}
