package contracts

// ExceptionHandlerContract defines the interface for centralized error handling.
// Mirrors AdonisJS's ExceptionHandler class.
type ExceptionHandlerContract interface {
	// Handle processes an error and sends an appropriate response.
	Handle(ctx HttpContextContract, err error)

	// Report logs the error for monitoring/debugging.
	Report(err error)
}
