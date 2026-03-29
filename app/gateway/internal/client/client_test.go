package client
package client

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFnvHash_Deterministic(t *testing.T) {
	h1 := fnvHash("abc123")
	h2 := fnvHash("abc123")
	assert.Equal(t, h1, h2)

	h3 := fnvHash("def456")
	assert.NotEqual(t, h1, h3)
}

func TestHashRouter_Pick_EmptyRing(t *testing.T) {
	hr := &hashRouter{
		ringMap: make(map[uint32]string),
		clients: make(map[string]*hashRouterEntry),
	}
	assert.Nil(t, hr.pick("any-key"))
}

func TestHashRouter_Pick_ConsistentRouting(t *testing.T) {
	hr := &hashRouter{
		ringMap: make(map[uint32]string),
		clients: make(map[string]*hashRouterEntry),
	}

	// Manually build a ring with two addresses
	addrs := []string{"10.0.0.1:9002", "10.0.0.2:9002"}
	for _, addr := range addrs {
		hr.clients[addr] = &hashRouterEntry{client: nil} // client is nil for test; we check addr routing
		for i := range virtualNodes {
			h := fnvHash(fmt.Sprintf("%s#%d", addr, i))
			hr.ring = append(hr.ring, h)
			hr.ringMap[h] = addr
		}
	}
	// Sort the ring
	sortRing(hr)

	// Same key always routes to same address
	addr1 := hr.pickAddr("file-md5-a")
	addr2 := hr.pickAddr("file-md5-a")
	assert.Equal(t, addr1, addr2, "consistent hashing should return same address for same key")

	// Different keys may route to different addresses (with high probability for 2 nodes)
	results := make(map[string]int)
	for i := 0; i < 100; i++ {
		addr := hr.pickAddr(fmt.Sprintf("key-%d", i))
		results[addr]++
	}
	assert.Len(t, results, 2, "100 keys should distribute across 2 nodes")
	for _, count := range results {
		assert.Greater(t, count, 10, "each node should get at least 10% of keys")
	}
}

func TestHashRouter_Pick_Rebalance(t *testing.T) {
	hr := &hashRouter{
		ringMap: make(map[uint32]string),
		clients: make(map[string]*hashRouterEntry),
	}

	// Start with 3 nodes
	addrs := []string{"10.0.0.1:9002", "10.0.0.2:9002", "10.0.0.3:9002"}
	rebuildRing(hr, addrs)

	// Record routing for many keys
	routingBefore := make(map[string]string, 50)
	for i := 0; i < 50; i++ {
		key := fmt.Sprintf("md5-%d", i)
		routingBefore[key] = hr.pickAddr(key)
	}

	// Remove one node (simulate failure)
	rebuildRing(hr, []string{"10.0.0.1:9002", "10.0.0.2:9002"})

	// Most keys should still route to the same node (consistent hashing property)
	unchanged := 0
	for i := 0; i < 50; i++ {
		key := fmt.Sprintf("md5-%d", i)
		if hr.pickAddr(key) == routingBefore[key] {
			unchanged++
		}
	}
	// With consistent hashing, removing 1 of 3 nodes should only remap ~1/3 of keys
	assert.Greater(t, unchanged, 25, "removing 1 of 3 nodes should keep most keys on the same node")
}

// --- Test helpers ---

func sortRing(hr *hashRouter) {
	// Use sort from the standard library
	n := len(hr.ring)
	for i := 1; i < n; i++ {
		key := hr.ring[i]
		j := i - 1
		for j >= 0 && hr.ring[j] > key {
			hr.ring[j+1] = hr.ring[j]
			j--
		}
		hr.ring[j+1] = key
	}
}

func rebuildRing(hr *hashRouter, addrs []string) {
	hr.mu.Lock()
	defer hr.mu.Unlock()

	hr.ring = hr.ring[:0]
	hr.ringMap = make(map[uint32]string, len(addrs)*virtualNodes)
	hr.clients = make(map[string]*hashRouterEntry, len(addrs))

	for _, addr := range addrs {
		hr.clients[addr] = &hashRouterEntry{client: nil}
		for i := range virtualNodes {
			h := fnvHash(fmt.Sprintf("%s#%d", addr, i))
			hr.ring = append(hr.ring, h)
			hr.ringMap[h] = addr
		}
	}
	sortRing(hr)
}

// pickAddr is a test helper that returns the address (not client) for a key.
func (hr *hashRouter) pickAddr(key string) string {
	hr.mu.RLock()
	defer hr.mu.RUnlock()

	if len(hr.ring) == 0 {
		return ""
	}

	h := fnvHash(key)
	// Binary search
	lo, hi := 0, len(hr.ring)
	for lo < hi {
		mid := lo + (hi-lo)/2
		if hr.ring[mid] < h {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	if lo >= len(hr.ring) {
		lo = 0
	}
	return hr.ringMap[hr.ring[lo]]
}
