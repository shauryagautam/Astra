package event

import (
	"context"
	"testing"
)

func BenchmarkEmitter_EmitPayload_Sync(b *testing.B) {
	e := New()
	ctx := context.Background()

	e.OnFunc("test.event", func(ctx context.Context, ev Event) error {
		return nil
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.EmitPayload(ctx, "test.event", "payload")
	}
}

func BenchmarkEmitter_EmitPayload_Async(b *testing.B) {
	e := New()
	ctx := context.Background()

	e.OnFunc("test.event", func(ctx context.Context, ev Event) error {
		return nil
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.EmitAsync(ctx, BaseEvent{EventName: "test.event", EventData: "payload"})
	}
}
