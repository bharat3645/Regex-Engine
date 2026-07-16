// File: events/emitter.go
package events

// EventEmitter defines a generic interface for emitting events.
// This decouples the core application logic from any specific UI framework like Wails.
type EventEmitter interface {
	// Emit sends an event with a given name and an optional payload.
	Emit(eventName string, payload ...interface{})
}
