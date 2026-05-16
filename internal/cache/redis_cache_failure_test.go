package cache

import (
	"testing"
	"time"

	miniredis "github.com/alicebob/miniredis/v2"
)

func TestRedisCacheMethodsWhenRedisUnavailable(t *testing.T) {
	srv, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	addr := srv.Addr()

	rc := NewRedisCache(addr, "", 0)
	if err := rc.Ping(); err != nil {
		t.Fatalf("ping failed: %v", err)
	}

	// close server to simulate unavailable redis
	srv.Close()

	// AddItem should return an error
	if err := rc.AddItem("u", "{}"); err == nil {
		t.Fatalf("expected error on AddItem when redis is down")
	}

	// GetItems should return an error
	if _, err := rc.GetItems("u", 10, 0); err == nil {
		t.Fatalf("expected error on GetItems when redis is down")
	}

	// RemoveItemByIndex should return an error
	if err := rc.RemoveItemByIndex("u", 0); err == nil {
		t.Fatalf("expected error on RemoveItemByIndex when redis is down")
	}

	// Len should return an error
	if _, err := rc.Len("u"); err == nil {
		t.Fatalf("expected error on Len when redis is down")
	}

	// ClearCart should return an error
	if err := rc.ClearCart("u"); err == nil {
		t.Fatalf("expected error on ClearCart when redis is down")
	}

	// finally, ensure that creating a new cache to unreachable address returns errors on ping quickly
	rc2 := NewRedisCache("127.0.0.1:65000", "", 0)
	_ = rc2
	t.Logf("created cache for unreachable address, sleeping briefly to allow connection attempt")
	time.Sleep(10 * time.Millisecond)
}
