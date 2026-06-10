package main

import "sync"

// decisionRouteStore maps pending approval/ask IDs to the tab that issued them so
// safe-tier ApproveTab/AnswerQuestionForTab cannot resolve a stale or cross-tab ID.
type decisionRouteStore struct {
	mu     sync.Mutex
	routes map[string]string // decisionID -> tabID (or clawDecisionTabID)
}

func newDecisionRouteStore() *decisionRouteStore {
	return &decisionRouteStore{routes: map[string]string{}}
}

func (s *decisionRouteStore) register(id, tabID string) {
	if s == nil || id == "" || tabID == "" {
		return
	}
	s.mu.Lock()
	if s.routes == nil {
		s.routes = map[string]string{}
	}
	s.routes[id] = tabID
	s.mu.Unlock()
}

func (s *decisionRouteStore) clear(id string) {
	if s == nil || id == "" {
		return
	}
	s.mu.Lock()
	delete(s.routes, id)
	s.mu.Unlock()
}

func (s *decisionRouteStore) clearAll() {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.routes = map[string]string{}
	s.mu.Unlock()
}

// resolve returns the tab that must handle decisionID. ok is false when the caller
// supplied an explicit tabID that does not match the registered route.
func (s *decisionRouteStore) resolve(requestedTab, decisionID, activeTab string) (tabID string, ok bool) {
	if decisionID == "" {
		return "", false
	}
	if s == nil {
		if requestedTab != "" {
			return requestedTab, true
		}
		if activeTab != "" {
			return activeTab, true
		}
		return "", false
	}
	s.mu.Lock()
	routeTab, registered := s.routes[decisionID]
	s.mu.Unlock()
	if !registered {
		if requestedTab != "" {
			return requestedTab, true
		}
		if activeTab != "" {
			return activeTab, true
		}
		return "", false
	}
	if requestedTab == "" {
		return routeTab, true
	}
	if requestedTab != routeTab {
		return "", false
	}
	return routeTab, true
}
