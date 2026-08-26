// Package lru will hold a generic LRU cache: O(1) Get/Put via a hash map
// plus doubly-linked list, with a capacity-based eviction policy.
// Write it test-first.
//
// Built in rung 5; used by the cache-aware router's eviction model.
package lru
