package queue

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m, goleak.IgnoreTopFunction("github.com/redis/go-redis/v9/internal/pool.(*ConnPool).reaper"))
}

type mockJob struct {
	BaseJob
	Payload string
}

func (j mockJob) Handle(ctx context.Context) error         { return nil }
func (j mockJob) OnFailure(ctx context.Context, err error) {}
func (j mockJob) MaxRetries() int                          { return 3 }
func (j mockJob) Queue() string                            { return "default" }
func (j mockJob) Timeout() time.Duration                   { return time.Second }

func TestJobHelpers(t *testing.T) {
	t.Run("jobTypeName", func(t *testing.T) {
		assert.Equal(t, "mockJob", jobTypeName(mockJob{}))
		assert.Equal(t, "mockJob", jobTypeName(&mockJob{}))
	})

	t.Run("normalizeQueuePrefix", func(t *testing.T) {
		assert.Equal(t, "astra", normalizeQueuePrefix(""))
		assert.Equal(t, "astra", normalizeQueuePrefix("queue"))
		assert.Equal(t, "my-app", normalizeQueuePrefix("my-app"))
		assert.Equal(t, "my-app", normalizeQueuePrefix("my-app:queue"))
	})

	t.Run("newQueueEnvelope", func(t *testing.T) {
		job := &mockJob{Payload: "test"}
		envelope, err := newQueueEnvelope(context.Background(), "Mock", job, 1)
		require.NoError(t, err)
		assert.Equal(t, "Mock", envelope.JobType)
		assert.Equal(t, "default", envelope.Queue)
		assert.Equal(t, 1, envelope.Attempts)
		assert.Equal(t, 3, envelope.MaxRetries) // default

		var decoded mockJob
		err = json.Unmarshal([]byte(envelope.Payload), &decoded)
		require.NoError(t, err)
		assert.Equal(t, "test", decoded.Payload)
	})
}

func TestEnvelopes(t *testing.T) {
}
