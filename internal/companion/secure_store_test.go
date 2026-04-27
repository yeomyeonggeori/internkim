package companion

import (
	"context"
	"testing"
)

func TestMemorySecureStoreRoundTrip(t *testing.T) {
	store := NewMemorySecureStore()
	if errorValue := store.Put(context.Background(), "key-1", "secret-1"); errorValue != nil {
		t.Fatal(errorValue)
	}
	secret, errorValue := store.Get(context.Background(), "key-1")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if secret != "secret-1" {
		t.Fatalf("unexpected secret: %s", secret)
	}
	if errorValue := store.Delete(context.Background(), "key-1"); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := store.Get(context.Background(), "key-1"); errorValue == nil {
		t.Fatal("expected deleted secret to be unavailable")
	}
}
