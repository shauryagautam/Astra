// Package core provides the fundamental building blocks of an Astra application,
// including the IoC Container and the Application lifecycle manager.
//
// Astra applications are built around the concept of Service Providers which
// allow for modular, decoupled growth of the framework and user applications.
//
// Core responsibilities:
//   - Container: Typed Dependency Injection and service resolution.
//   - Application: Managing the startup and shutdown sequence (hooks).
//   - Configuration: Unified access to environment variables and config files.
//   - Logger: Standardized slog-based logging across all framework components.
package engine
