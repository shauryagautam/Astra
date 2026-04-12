package event

import (
	"time"
)

// Server lifecycle events
type ServerStartingEvent struct {
	Addr string
}

func (e ServerStartingEvent) Name() string { return "server.starting" }
func (e ServerStartingEvent) Data() any    { return e }

type ServerStartedEvent struct {
	Addr string
}

func (e ServerStartedEvent) Name() string { return "server.started" }
func (e ServerStartedEvent) Data() any    { return e }

type ServerStoppingEvent struct {
	Signal string
}

func (e ServerStoppingEvent) Name() string { return "server.stopping" }
func (e ServerStoppingEvent) Data() any    { return e }

type ServerStoppedEvent struct{}

func (e ServerStoppedEvent) Name() string { return "server.stopped" }
func (e ServerStoppedEvent) Data() any    { return e }

// Request lifecycle events
type RequestStartedEvent struct {
	Method string
	Path   string
	IP     string
}

func (e RequestStartedEvent) Name() string { return "request.started" }
func (e RequestStartedEvent) Data() any    { return e }

type RequestFinishedEvent struct {
	Method   string
	Path     string
	Status   int
	Duration time.Duration
}

func (e RequestFinishedEvent) Name() string { return "request.finished" }
func (e RequestFinishedEvent) Data() any    { return e }

// DB events
type QueryExecutedEvent struct {
	SQL      string
	Args     []any
	Duration time.Duration
	Error    error
}

func (e QueryExecutedEvent) Name() string { return "db.query_executed" }
func (e QueryExecutedEvent) Data() any    { return e }

// Queue events
type JobQueuedEvent struct {
	ID      string
	JobType string
	Queue   string
}

func (e JobQueuedEvent) Name() string { return "queue.job_queued" }
func (e JobQueuedEvent) Data() any    { return e }

type JobProcessingEvent struct {
	ID      string
	JobType string
}

func (e JobProcessingEvent) Name() string { return "queue.job_processing" }
func (e JobProcessingEvent) Data() any    { return e }

type JobProcessedEvent struct {
	ID       string
	JobType  string
	Duration time.Duration
}

func (e JobProcessedEvent) Name() string { return "queue.job_processed" }
func (e JobProcessedEvent) Data() any    { return e }

type JobFailedEvent struct {
	ID      string
	JobType string
	Error   error
}

func (e JobFailedEvent) Name() string { return "queue.job_failed" }
func (e JobFailedEvent) Data() any    { return e }

// Mail events
type MailSendingEvent struct {
	To      string
	Subject string
}

func (e MailSendingEvent) Name() string { return "mail.sending" }
func (e MailSendingEvent) Data() any    { return e }

type MailSentEvent struct {
	To      string
	Subject string
}

func (e MailSentEvent) Name() string { return "mail.sent" }
func (e MailSentEvent) Data() any    { return e }

// Redis events
type RedisCommandExecutedEvent struct {
	Command  string
	Args     []any
	Duration time.Duration
	Error    error
}

func (e RedisCommandExecutedEvent) Name() string { return "redis.command_executed" }
func (e RedisCommandExecutedEvent) Data() any    { return e }
