package oauth

import (
	"github.com/EnockYator/go-oauth/internal/modules/auth/domain"
)

// Registry is the application's catalogue of configured providers.
//
// Registry is populated during startup and read-only afterwards, so no
// mutex is required.
type Registry struct {
	providers map[auth.ProviderID]Provider
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{providers: make(map[auth.ProviderID]Provider)}
}

// Register adds a provider. Later registrations with the same ID replace
// earlier ones, which is useful for tests.
func (r *Registry) Register(p Provider) {
	r.providers[p.ID()] = p
}

// Get returns the provider with the given ID.
func (r *Registry) Get(id auth.ProviderID) (Provider, bool) {
	p, ok := r.providers[id]
	return p, ok
}

// IDs returns the registered provider IDs. The slice is a copy; callers
// cannot mutate the registry.
func (r *Registry) IDs() []auth.ProviderID {
	ids := make([]auth.ProviderID, 0, len(r.providers))
	for id := range r.providers {
		ids = append(ids, id)
	}
	return ids
}
