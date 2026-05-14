package cache

import (
	"testing"

	miniredis "github.com/alicebob/miniredis/v2"
)

func TestRedisCacheCartLifecycle(t *testing.T) {
	srv, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	t.Cleanup(srv.Close)

	cache := NewRedisCache(srv.Addr(), "", 0)
	if err := cache.Ping(); err != nil {
		t.Fatalf("ping failed: %v", err)
	}

	userID := "user-123"
	if err := cache.AddItem(userID, `{"survey_id":"s1"}`); err != nil {
		t.Fatalf("AddItem failed: %v", err)
	}
	if err := cache.AddItem(userID, `{"survey_id":"s2"}`); err != nil {
		t.Fatalf("AddItem failed: %v", err)
	}

	count, err := cache.Len(userID)
	if err != nil {
		t.Fatalf("Len failed: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected len 2, got %d", count)
	}

	items, err := cache.GetItems(userID, 10, 0)
	if err != nil {
		t.Fatalf("GetItems failed: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0] != `{"survey_id":"s2"}` || items[1] != `{"survey_id":"s1"}` {
		t.Fatalf("unexpected item order: %#v", items)
	}

	if err := cache.RemoveItemByIndex(userID, 0); err != nil {
		t.Fatalf("RemoveItemByIndex failed: %v", err)
	}
	items, err = cache.GetItems(userID, 10, 0)
	if err != nil {
		t.Fatalf("GetItems after remove failed: %v", err)
	}
	if len(items) != 1 || items[0] != `{"survey_id":"s1"}` {
		t.Fatalf("unexpected items after remove: %#v", items)
	}

	if err := cache.ClearCart(userID); err != nil {
		t.Fatalf("ClearCart failed: %v", err)
	}
	count, err = cache.Len(userID)
	if err != nil {
		t.Fatalf("Len after clear failed: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected len 0 after clear, got %d", count)
	}
}

func TestRedisCachePagination(t *testing.T) {
	srv, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	t.Cleanup(srv.Close)

	cache := NewRedisCache(srv.Addr(), "", 0)
	userID := "user-456"
	for i := 0; i < 5; i++ {
		if err := cache.AddItem(userID, string(rune('a'+i))); err != nil {
			t.Fatalf("AddItem failed: %v", err)
		}
	}

	items, err := cache.GetItems(userID, 2, 1)
	if err != nil {
		t.Fatalf("GetItems failed: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
}
