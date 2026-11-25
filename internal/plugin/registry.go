package plugin

import (
	"fmt"
	"sync"
)

// Registry holds south/north plugin factories.
type Registry struct {
	mu     sync.RWMutex
	south  map[string]SouthDriver
	north  map[string]NorthChannel
}

// NewRegistry creates an empty registry.
func NewRegistry() *Registry {
	return &Registry{
		south: make(map[string]SouthDriver),
		north: make(map[string]NorthChannel),
	}
}

// RegisterSouth registers a south driver (last write wins).
func (r *Registry) RegisterSouth(d SouthDriver) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.south[d.Type()] = d
}

// RegisterNorth registers a north channel.
func (r *Registry) RegisterNorth(c NorthChannel) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.north[c.Type()] = c
}

// South returns a driver by type.
func (r *Registry) South(typ string) (SouthDriver, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	d, ok := r.south[typ]
	if !ok {
		return nil, fmt.Errorf("south driver %q not registered", typ)
	}
	return d, nil
}

// North returns a channel by type.
func (r *Registry) North(typ string) (NorthChannel, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	c, ok := r.north[typ]
	if !ok {
		return nil, fmt.Errorf("north channel %q not registered", typ)
	}
	return c, nil
}

// List returns registered plugin types.
func (r *Registry) List() (south []string, north []string) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for k := range r.south {
		south = append(south, k)
	}
	for k := range r.north {
		north = append(north, k)
	}
	return south, north
}
