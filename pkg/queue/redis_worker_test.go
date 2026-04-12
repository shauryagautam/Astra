package queue

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRedisWorkerQueuePollOrderCoversAllQueues(t *testing.T) {
	worker := NewRedisWorker(nil, "astra", []string{"default", "mail", "reports"}, nil)

	require.Equal(t, []string{"default", "mail", "reports"}, worker.queuePollOrder(0))
	require.Equal(t, []string{"mail", "reports", "default"}, worker.queuePollOrder(1))
	require.Equal(t, []string{"reports", "default", "mail"}, worker.queuePollOrder(2))
	require.Equal(t, []string{"mail", "reports", "default"}, worker.queuePollOrder(4))
}
