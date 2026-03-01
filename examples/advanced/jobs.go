package main

import (
	"context"
	"fmt"

	"github.com/shaurya/astra/contracts"
)

func registerAdvancedJobs(app contracts.ApplicationContract) {
	registry := app.Use("JobRegistry").(contracts.JobRegistry)

	// Register a custom job handler
	registry.Register("app:notification", func(data []byte) error {
		fmt.Printf("[queue:worker] Processing notification job: %s\n", string(data))
		return nil
	})
}

// NotificationJob represents a custom job.
type NotificationJob struct {
	Message string `json:"message"`
}

func (j *NotificationJob) Execute(ctx context.Context) error {
	fmt.Printf("Executing notification: %s\n", j.Message)
	return nil
}

func (j *NotificationJob) DisplayName() string {
	return "app:notification"
}
