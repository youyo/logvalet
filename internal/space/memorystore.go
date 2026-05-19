package space

import (
	"context"
	"sync"
	"time"
)

// MemoryStore は Store + NonceStore のインメモリ実装。テスト・開発用。並行安全。
type MemoryStore struct {
	mu          sync.RWMutex
	spaces      map[string]map[string]SpaceRegistration
	preferences map[string]UserPreference
	nonces      map[string]map[string]time.Time
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		spaces:      make(map[string]map[string]SpaceRegistration),
		preferences: make(map[string]UserPreference),
		nonces:      make(map[string]map[string]time.Time),
	}
}

func (s *MemoryStore) List(ctx context.Context, userID string) ([]SpaceRegistration, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]SpaceRegistration, 0)
	userSpaces, ok := s.spaces[userID]
	if !ok {
		return result, nil
	}
	for _, reg := range userSpaces {
		result = append(result, reg)
	}
	return result, nil
}

func (s *MemoryStore) Get(ctx context.Context, userID, alias string) (*SpaceRegistration, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	userSpaces, ok := s.spaces[userID]
	if !ok {
		return nil, nil
	}
	reg, ok := userSpaces[alias]
	if !ok {
		return nil, nil
	}
	copy := reg
	return &copy, nil
}

func (s *MemoryStore) Upsert(ctx context.Context, reg *SpaceRegistration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.spaces[reg.UserID] == nil {
		s.spaces[reg.UserID] = make(map[string]SpaceRegistration)
	}
	s.spaces[reg.UserID][reg.Alias] = *reg
	return nil
}

func (s *MemoryStore) Delete(ctx context.Context, userID, alias string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if userSpaces, ok := s.spaces[userID]; ok {
		delete(userSpaces, alias)
	}
	return nil
}

func (s *MemoryStore) GetPreference(ctx context.Context, userID string) (*UserPreference, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	pref, ok := s.preferences[userID]
	if !ok {
		return nil, nil
	}
	copy := pref
	return &copy, nil
}

func (s *MemoryStore) PutPreference(ctx context.Context, pref *UserPreference) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.preferences[pref.UserID] = *pref
	return nil
}

func (s *MemoryStore) Close() error {
	return nil
}

// NonceStore 実装

func (s *MemoryStore) Store(ctx context.Context, userID, nonce string, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.nonces[userID] == nil {
		s.nonces[userID] = make(map[string]time.Time)
	}
	s.nonces[userID][nonce] = time.Now().Add(ttl)
	return nil
}

func (s *MemoryStore) Consume(ctx context.Context, userID, nonce string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	userNonces, ok := s.nonces[userID]
	if !ok {
		return ErrNonceAlreadyUsed
	}
	exp, exists := userNonces[nonce]
	if !exists {
		return ErrNonceAlreadyUsed
	}
	if time.Now().After(exp) {
		delete(userNonces, nonce)
		return ErrNonceAlreadyUsed
	}
	delete(userNonces, nonce)
	return nil
}
