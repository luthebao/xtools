package events

import (
	"context"
	"sync"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"xtools/internal/ports"
)

// WailsEventBus implements EventBus using Wails runtime
type WailsEventBus struct {
	ctx       context.Context
	mu        sync.RWMutex
	handlers  map[string][]ports.EventHandler
}

// NewWailsEventBus creates a new Wails-based event bus
func NewWailsEventBus(ctx context.Context) *WailsEventBus {
	return &WailsEventBus{
		ctx:      ctx,
		handlers: make(map[string][]ports.EventHandler),
	}
}

// SetContext updates the Wails context
func (e *WailsEventBus) SetContext(ctx context.Context) {
	e.ctx = ctx
}

// Emit sends an event to the frontend
func (e *WailsEventBus) Emit(eventName string, data interface{}) {
	if e.ctx == nil {
		return
	}

	// Emit to frontend via Wails runtime
	runtime.EventsEmit(e.ctx, eventName, data)

	// Also notify internal handlers
	e.notifyHandlers(eventName, data)
}

// EmitTo sends an event for a specific account
func (e *WailsEventBus) EmitTo(accountID string, eventName string, data interface{}) {
	if e.ctx == nil {
		return
	}

	// Add account context to event name
	fullEventName := accountID + ":" + eventName

	runtime.EventsEmit(e.ctx, fullEventName, data)
	runtime.EventsEmit(e.ctx, eventName, data) // Also emit general event

	e.notifyHandlers(fullEventName, data)
	e.notifyHandlers(eventName, data)
}

// Subscribe allows backend components to listen to events
func (e *WailsEventBus) Subscribe(eventName string, handler ports.EventHandler) func() {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.handlers[eventName] = append(e.handlers[eventName], handler)

	// Return unsubscribe function
	return func() {
		e.Unsubscribe(eventName, handler)
	}
}

// Unsubscribe removes an event handler
func (e *WailsEventBus) Unsubscribe(eventName string, handler ports.EventHandler) {
	e.mu.Lock()
	defer e.mu.Unlock()

	handlers := e.handlers[eventName]
	for i, h := range handlers {
		// Compare function pointers (works for same reference)
		if &h == &handler {
			e.handlers[eventName] = append(handlers[:i], handlers[i+1:]...)
			break
		}
	}
}

func (e *WailsEventBus) notifyHandlers(eventName string, data interface{}) {
	e.mu.RLock()
	handlers := make([]ports.EventHandler, len(e.handlers[eventName]))
	copy(handlers, e.handlers[eventName])
	e.mu.RUnlock()

	for _, h := range handlers {
		go h(data) // Run handlers in goroutines to avoid blocking
	}
}

// EmitAccountStatus emits an account status event
func (e *WailsEventBus) EmitAccountStatus(accountID, status, message string, err error) {
	errStr := ""
	if err != nil {
		errStr = err.Error()
	}

	e.Emit(ports.EventAccountUpdated, ports.AccountStatusEvent{
		AccountID: accountID,
		Status:    status,
		Message:   message,
		Error:     errStr,
	})
}

// EmitWorkerStatus emits a worker status event
func (e *WailsEventBus) EmitWorkerStatus(accountID, workerType string, isRunning bool, lastActivity, nextRun string) {
	e.Emit(ports.EventWorkerStatus, ports.WorkerStatusEvent{
		AccountID:    accountID,
		WorkerType:   workerType,
		IsRunning:    isRunning,
		LastActivity: lastActivity,
		NextRun:      nextRun,
	})
}

// EmitReplyEvent emits a reply-related event
func (e *WailsEventBus) EmitReplyEvent(eventName, accountID string, reply interface{}, tweetID string, err error) {
	errStr := ""
	if err != nil {
		errStr = err.Error()
	}

	e.Emit(eventName, ports.ReplyEvent{
		AccountID: accountID,
		Reply:     reply,
		TweetID:   tweetID,
		Error:     errStr,
	})
}

// Ensure WailsEventBus implements EventBus interface
var _ ports.EventBus = (*WailsEventBus)(nil)
