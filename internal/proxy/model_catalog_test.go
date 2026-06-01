package proxy

import (
	"context"
	"testing"
)

func TestStaticModelCatalogLooksUpProviderModelOnly(t *testing.T) {
	t.Parallel()

	catalog := NewStaticModelCatalog([]ProviderModel{{
		Provider:       "ark",
		CanonicalModel: "deepseek-v4-flash",
		ProviderModel:  "ark-compatible-name",
	}})

	if _, ok := catalog.LookupProviderModel(context.Background(), "ark", "ark-compatible-name"); !ok {
		t.Fatal("expected provider_model lookup to match")
	}
	if _, ok := catalog.LookupProviderModel(context.Background(), "ark", "deepseek-v4-flash"); ok {
		t.Fatal("canonical model should not match before name conversion is implemented")
	}
}
