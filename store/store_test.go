package store

import (
	"bytes"
	"os"
	"testing"
)

// These tests require a running Redis instance reachable using the project's
// redisclient helper (REDIS_ADDR / REDIS_PASSWORD). If Redis is not
// available the tests will be skipped.

func TestStoreSetGetListDel(t *testing.T) {
	s, err := New("test:store:")
	if err != nil {
		t.Skipf("redis not available: %v", err)
	}
	defer s.Close()

	key := []byte("k1")
	val := []byte("hello world")

	if err := s.Set(key, val); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	got, err := s.Get(key)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if !bytes.Equal(got, val) {
		t.Fatalf("Get returned unexpected value: %q", string(got))
	}

	// List should contain our kv
	list, err := s.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	found := false
	for _, kv := range list {
		if bytes.Equal(kv.Key, key) && bytes.Equal(kv.Value, val) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("List did not contain stored key")
	}

	// Delete and ensure Get fails
	if err := s.Del(key); err != nil {
		t.Fatalf("Del failed: %v", err)
	}
	if _, err := s.Get(key); err == nil {
		t.Fatalf("expected Get after Del to fail")
	}

	// also ensure we can create a store with a file-backed env (sanity)
	if _, ok := os.LookupEnv("REDIS_ADDR"); !ok {
		t.Log("REDIS_ADDR not set locally - tests used the default environment")
	}
}
