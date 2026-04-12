package graphql

import (
	"github.com/graph-gophers/dataloader/v7"
)

// NewDataLoader creates a new dataloader with default settings.
func NewDataLoader[K comparable, V any](batchFn dataloader.BatchFunc[K, V]) *dataloader.Loader[K, V] {
	return dataloader.NewBatchedLoader(batchFn)
}

// BatchResult is a helper to wrap values and errors for Dataloader.
type BatchResult[V any] struct {
	Value V
	Error error
}
