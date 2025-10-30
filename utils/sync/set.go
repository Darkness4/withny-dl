// Package sync provides a thread-safe structures.
package sync

import "sync"

// Set is a thread-safe set implementation using a map.
type Set[K comparable] struct {
	items map[K]struct{}
	mu    sync.RWMutex
}

// NewSet creates a new thread-safe set.
func NewSet[K comparable]() *Set[K] {
	return &Set[K]{
		items: make(map[K]struct{}),
	}
}

// Contains checks if an item exists in the set.
func (s *Set[K]) Contains(key K) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.items[key]
	return ok
}

// Set sets an item to the set.
func (s *Set[K]) Set(key K) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[key] = struct{}{}
}

// Release removes an item from the set.
func (s *Set[K]) Release(key K) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.items, key)
}

// Len returns the number of items in the set.
func (s *Set[K]) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.items)
}
