// Purpose: Item resource type and in-memory store for the items API example.

package items

import (
	"strconv"
	"sync"
)

// Item is the resource managed by the items API example.
type Item struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Note string `json:"note,omitempty"`
}

// CreateItemRequest is the JSON body expected by POST /api/v1/items.
type CreateItemRequest struct {
	Name string `json:"name"`
	Note string `json:"note,omitempty"`
}

// ItemStore is a goroutine-safe in-memory store for Items.
type ItemStore struct {
	mu    sync.Mutex
	items map[string]*Item
	seq   int
}

// NewItemStore returns an empty ItemStore.
func NewItemStore() *ItemStore {
	return &ItemStore{items: make(map[string]*Item)}
}

// Create adds an item to the store and returns it with an assigned ID.
func (s *ItemStore) Create(req CreateItemRequest) *Item {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seq++
	id := strconv.Itoa(s.seq)
	item := &Item{ID: id, Name: req.Name, Note: req.Note}
	s.items[id] = item
	return item
}

// List returns all items, up to limit. A limit of zero returns all items.
func (s *ItemStore) List(limit int) []*Item {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]*Item, 0, len(s.items))
	for _, item := range s.items {
		result = append(result, item)
		if limit > 0 && len(result) >= limit {
			break
		}
	}
	return result
}

// Get returns the item with the given ID, or false if it does not exist.
func (s *ItemStore) Get(id string) (*Item, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.items[id]
	return item, ok
}

// Delete removes the item with the given ID and reports whether it existed.
func (s *ItemStore) Delete(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.items[id]
	if ok {
		delete(s.items, id)
	}
	return ok
}
