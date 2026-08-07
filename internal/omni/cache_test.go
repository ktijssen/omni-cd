package omni

// Internal (same-package) test: the watch caches are package-level state and the
// TTL is measured against unexported timestamps, so expiry has to be simulated
// by winding those timestamps back rather than by sleeping for CacheTTL.

import (
	"testing"
	"time"
)

func TestCacheFresh(t *testing.T) {
	if cacheFresh(time.Time{}) {
		t.Error("zero time should not be fresh (cache never populated)")
	}
	if !cacheFresh(time.Now()) {
		t.Error("just-stamped cache should be fresh")
	}
	if !cacheFresh(time.Now().Add(-CacheTTL + time.Second)) {
		t.Error("cache just inside CacheTTL should be fresh")
	}
	if cacheFresh(time.Now().Add(-CacheTTL - time.Second)) {
		t.Error("cache past CacheTTL should not be fresh")
	}
}

func TestClusterCache_NotOKBeforePopulate(t *testing.T) {
	ClearCache()
	if _, _, ok := GetCachedClusterIDsWithPhases(); ok {
		t.Error("cache should not be ok before it has ever been populated")
	}
}

func TestClusterCache_OKWhenFresh(t *testing.T) {
	ClearCache()
	CacheClusterSnapshot([]string{"a", "b"}, map[string]bool{"b": true})

	ids, td, ok := GetCachedClusterIDsWithPhases()
	if !ok {
		t.Fatal("cache should be ok immediately after being populated")
	}
	if len(ids) != 2 {
		t.Errorf("got %d ids, want 2", len(ids))
	}
	if !td["b"] {
		t.Error("tearing-down flag for \"b\" was lost")
	}
}

func TestClusterCache_ExpiresPastTTL(t *testing.T) {
	ClearCache()
	CacheClusterSnapshot([]string{"a"}, nil)

	// Wind the stamp back past the TTL — this is the stalled-watch case: the data
	// is still in memory but no longer trustworthy, so callers must be told to go
	// ask Omni directly instead of being served frozen contents.
	clusterCacheMu.Lock()
	clusterCacheAt = time.Now().Add(-CacheTTL - time.Second)
	clusterCacheMu.Unlock()

	ids, td, ok := GetCachedClusterIDsWithPhases()
	if ok {
		t.Error("cache older than CacheTTL must report not-ok")
	}
	if ids != nil || td != nil {
		t.Errorf("expired cache must return nil contents, got ids=%v td=%v", ids, td)
	}
}

func TestClusterCache_RefreshRenewsTTL(t *testing.T) {
	ClearCache()
	CacheClusterSnapshot([]string{"a"}, nil)
	clusterCacheMu.Lock()
	clusterCacheAt = time.Now().Add(-CacheTTL - time.Second)
	clusterCacheMu.Unlock()

	// A reconnected watch re-delivers its contents — that must make the cache
	// usable again without any other intervention.
	CacheClusterSnapshot([]string{"a", "c"}, nil)
	ids, _, ok := GetCachedClusterIDsWithPhases()
	if !ok {
		t.Fatal("cache should be ok again after a fresh snapshot")
	}
	if len(ids) != 2 {
		t.Errorf("got %d ids, want 2", len(ids))
	}
}

func TestClusterCache_ReturnsCopies(t *testing.T) {
	ClearCache()
	CacheClusterSnapshot([]string{"a"}, map[string]bool{"a": true})

	ids, td, _ := GetCachedClusterIDsWithPhases()
	ids[0] = "mutated"
	td["a"] = false

	ids2, td2, _ := GetCachedClusterIDsWithPhases()
	if ids2[0] != "a" {
		t.Errorf("caller mutated the cached ID slice: got %q", ids2[0])
	}
	if !td2["a"] {
		t.Error("caller mutated the cached tearing-down map")
	}
}

func TestMachineClassCache_ExpiresPastTTL(t *testing.T) {
	ClearCache()
	CacheMachineClassSnapshot(map[string]string{"mc1": "yaml"})

	if _, ok := GetCachedMachineClasses(); !ok {
		t.Fatal("cache should be ok immediately after being populated")
	}

	mcCacheMu.Lock()
	mcCacheAt = time.Now().Add(-CacheTTL - time.Second)
	mcCacheMu.Unlock()

	content, ok := GetCachedMachineClasses()
	if ok {
		t.Error("cache older than CacheTTL must report not-ok")
	}
	if content != nil {
		t.Errorf("expired cache must return nil contents, got %v", content)
	}
}

func TestClearCache_ResetsBoth(t *testing.T) {
	CacheClusterSnapshot([]string{"a"}, nil)
	CacheMachineClassSnapshot(map[string]string{"mc1": "yaml"})

	ClearCache()

	if _, _, ok := GetCachedClusterIDsWithPhases(); ok {
		t.Error("cluster cache should not be ok after ClearCache")
	}
	if _, ok := GetCachedMachineClasses(); ok {
		t.Error("machine class cache should not be ok after ClearCache")
	}
}
