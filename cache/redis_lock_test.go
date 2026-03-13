package cache

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
)

func setupRedisLocker(t *testing.T) (*RedisLocker, *miniredis.Miniredis, *goredis.Client) {
	t.Helper()

	server, err := miniredis.Run()
	if err != nil {
		t.Fatalf("run miniredis: %v", err)
	}

	client := goredis.NewClient(&goredis.Options{Addr: server.Addr()})
	return NewRedisLocker(client, "astra:lock:"), server, client
}

func TestRedisLockerAcquireAndRelease(t *testing.T) {
	locker, server, client := setupRedisLocker(t)
	defer server.Close()
	defer client.Close()

	lock, err := locker.Acquire(context.Background(), "resource", time.Second)
	if err != nil {
		t.Fatalf("acquire lock: %v", err)
	}

	if err := lock.Release(context.Background()); err != nil {
		t.Fatalf("release lock: %v", err)
	}
}

func TestRedisLockerNotAcquired(t *testing.T) {
	locker, server, client := setupRedisLocker(t)
	defer server.Close()
	defer client.Close()

	if _, err := locker.Acquire(context.Background(), "resource", time.Second); err != nil {
		t.Fatalf("first acquire: %v", err)
	}

	_, err := locker.Acquire(context.Background(), "resource", time.Second)
	if !errors.Is(err, ErrLockNotAcquired) {
		t.Fatalf("expected ErrLockNotAcquired, got %v", err)
	}
}

func TestRedisLockerWithRetry(t *testing.T) {
	locker, server, client := setupRedisLocker(t)
	defer server.Close()
	defer client.Close()

	lock, err := locker.Acquire(context.Background(), "retry", time.Second)
	if err != nil {
		t.Fatalf("acquire lock: %v", err)
	}

	go func() {
		time.Sleep(40 * time.Millisecond)
		_ = lock.Release(context.Background())
	}()

	if _, err := locker.Acquire(context.Background(), "retry", time.Second, WithRetry(5, 20*time.Millisecond)); err != nil {
		t.Fatalf("acquire with retry: %v", err)
	}
}

func TestRedisLockTokenMismatch(t *testing.T) {
	_, server, client := setupRedisLocker(t)
	defer server.Close()
	defer client.Close()

	server.Set("astra:lock:mismatch", "their-token")
	server.SetTTL("astra:lock:mismatch", time.Second)

	lock := &RedisLock{
		client: client,
		key:    "astra:lock:mismatch",
		token:  "our-token",
	}

	if err := lock.Release(context.Background()); !errors.Is(err, ErrLockNotOwned) {
		t.Fatalf("expected ErrLockNotOwned on release, got %v", err)
	}
	if err := lock.Extend(context.Background(), 2*time.Second); !errors.Is(err, ErrLockNotOwned) {
		t.Fatalf("expected ErrLockNotOwned on extend, got %v", err)
	}
}

func TestRedisLockExtend(t *testing.T) {
	locker, server, client := setupRedisLocker(t)
	defer server.Close()
	defer client.Close()

	lock, err := locker.Acquire(context.Background(), "extend", 100*time.Millisecond)
	if err != nil {
		t.Fatalf("acquire lock: %v", err)
	}

	if err := lock.Extend(context.Background(), time.Second); err != nil {
		t.Fatalf("extend lock: %v", err)
	}
	if ttl := server.TTL("astra:lock:extend"); ttl < 900*time.Millisecond {
		t.Fatalf("expected ttl to increase, got %v", ttl)
	}
}

func TestRedisLockerContextCanceled(t *testing.T) {
	locker, server, client := setupRedisLocker(t)
	defer server.Close()
	defer client.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := locker.Acquire(ctx, "cancel", time.Second, WithRetry(2, 10*time.Millisecond)); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled on acquire, got %v", err)
	}

	lock, err := locker.Acquire(context.Background(), "owned", time.Second)
	if err != nil {
		t.Fatalf("acquire lock: %v", err)
	}

	if err := lock.Release(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled on release, got %v", err)
	}
	if err := lock.Extend(ctx, time.Second); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled on extend, got %v", err)
	}
}
