package mail

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/shaurya/adonis/contracts"
)

// MailJob represents a job for background email delivery.
type MailJob struct {
	Message contracts.MailMessage `json:"message"`
	Mailer  string                `json:"mailer"`
	manager contracts.MailerContract
}

// Execute handles the job by calling the mailer's Send method.
func (j *MailJob) Execute(ctx context.Context) error {
	if j.manager == nil {
		return fmt.Errorf("mailer manager not provided to job")
	}

	return j.manager.Use(j.Mailer).Send(ctx, j.Message)
}

// DisplayName returns the internal job name.
func (j *MailJob) DisplayName() string {
	return "adonis:mail"
}

// HandleMailJob is the raw handler for the queue system.
func HandleMailJob(data []byte, manager contracts.MailerContract) error {
	var job MailJob
	if err := json.Unmarshal(data, &job); err != nil {
		return fmt.Errorf("failed to unmarshal mail job: %w", err)
	}

	job.manager = manager
	return job.Execute(context.Background())
}
