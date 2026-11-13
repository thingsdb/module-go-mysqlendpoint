package main

import "sync"

type UserScopeLookup struct {
	mu     sync.RWMutex
	scopes map[string]string
}

var userScopes = &UserScopeLookup{
	scopes: make(map[string]string),
}

// Get returns the scope for a username (and whether it was found).
func (l *UserScopeLookup) Get(username string) (string, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	scope, ok := l.scopes[username]
	return scope, ok
}

// Replace atomically clears the old lookup and sets the new mapping.
func (l *UserScopeLookup) Replace(next map[string]string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.scopes = next
}
