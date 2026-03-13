package cache

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
)

func setupRedisStore(t *testing.T) (*RedisStore, *miniredis.Miniredis, *goredis.Client) {
	t.Helper()

	server, err := miniredis.Run()
	if err != nil {
		t.Fatalf("run miniredis: %v", err)
	}

	client := goredis.NewClient(&goredis.Options{Addr: server.Addr()})
	store := NewRedisStore(client, "astra:cache:")
	return store, server, client
}

func TestRedisStoreGetSetAndMiss(t *testing.T) {
	store, server, client := setupRedisStore(t)
	defer server.Close()
	defer client.Close()

	ctx := context.Background()
	if err := store.Set(ctx, "alpha", "bravo", time.Second); err != nil {
		t.Fatalf("set key: %v", err)
	}

	value, err := store.Get(ctx, "alpha")
	if err != nil {
		t.Fatalf("get key: %v", err)
	}
	if value != "bravo" {
		t.Fatalf("expected bravo, got %q", value)
	}

	_, err = store.Get(ctx, "missing")
	if !errors.Is(err, ErrCacheMiss) {
		t.Fatalf("expected ErrCacheMiss, got %v", err)
	}
}

func TestRedisStoreFlushHonorsPrefix(t *testing.T) {
	store, server, client := setupRedisStore(t)
	defer server.Close()
	defer client.Close()

	ctx := context.Background()
	if err := store.Set(ctx, "one", "1", 0); err != nil {
		t.Fatalf("set one: %v", err)
	}
	if err := store.Set(ctx, "two", "2", 0); err != nil {
		t.Fatalf("set two: %v", err)
	}
	server.Set("other:key", "value")

	if err := store.Flush(ctx); err != nil {
		t.Fatalf("flush store: %v", err)
	}

	if server.Exists("other:key") == false {
		t.Fatal("flush removed unrelated key")
	}
}

func TestRedisStoreRemember(t *testing.T) {
	store, server, client := setupRedisStore(t)
	defer server.Close()
	defer client.Close()

	ctx := context.Background()
	calls := 0
	fn := func() (string, error) {
		calls++
		return "computed", nil
	}

	value, err := store.Remember(ctx, "remember", time.Minute, fn)
	if err != nil {
		t.Fatalf("remember miss: %v", err)
	}
	if value != "computed" || calls != 1 {
		t.Fatalf("unexpected remember result: value=%q calls=%d", value, calls)
	}

	value, err = store.Remember(ctx, "remember", time.Minute, fn)
	if err != nil {
		t.Fatalf("remember hit: %v", err)
	}
	if value != "computed" || calls != 1 {
		t.Fatalf("unexpected cached remember result: value=%q calls=%d", value, calls)
	}
}

func TestRedisStoreGetManySetMany(t *testing.T) {
	store, server, client := setupRedisStore(t)
	defer server.Close()
	defer client.Close()

	ctx := context.Background()
	if err := store.SetMany(ctx, map[string]any{
		"one": "1",
		"two": 2,
	}, 0); err != nil {
		t.Fatalf("set many: %v", err)
	}

	values, err := store.GetMany(ctx, []string{"one", "two", "three"})
	if err != nil {
		t.Fatalf("get many: %v", err)
	}

	if values["one"] != "1" {
		t.Fatalf("expected one=1, got %q", values["one"])
	}
	if values["two"] != "2" {
		t.Fatalf("expected two=2, got %q", values["two"])
	}
	if _, ok := values["three"]; ok {
		t.Fatal("unexpected missing key in results")
	}
}

func TestRedisStoreContextCanceled(t *testing.T) {
	store, server, client := setupRedisStore(t)
	defer server.Close()
	defer client.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := store.Get(ctx, "alpha"); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled for get, got %v", err)
	}
	if err := store.Set(ctx, "alpha", "bravo", 0); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled for set, got %v", err)
	}
	if err := store.Delete(ctx, "alpha"); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled for delete, got %v", err)
	}
	if _, err := store.Has(ctx, "alpha"); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled for has, got %v", err)
	}
	if err := store.Flush(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled for flush, got %v", err)
	}
}
