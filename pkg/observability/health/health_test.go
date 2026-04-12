package health

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHealthChecker(t *testing.T) {
	checker := NewHealthChecker()

	checker.Register("test_ok", func(ctx context.Context) error {
		return nil
	})

	checker.Register("test_err", func(ctx context.Context) error {
		return errors.New("boom")
	})

	reportObj := checker.Report(context.Background(), "L1")
	report := reportObj.(HealthReport)

	assert.Equal(t, StatusError, report.Status)
	assert.Equal(t, StatusOk, report.Components["test_ok"].Status)
	assert.Equal(t, StatusError, report.Components["test_err"].Status)
	assert.Equal(t, "boom", report.Components["test_err"].Message)
}
