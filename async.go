package configcat

import (
	"sync"
	"sync/atomic"
	"time"
)

// Describes an error used when the given async operation is cancelled.
type CancelledError struct { }

// Error returns with the error message.
func (c *CancelledError) Error() string {
	return "The task was cancelled!"
}

// Describes an object which used to control asynchronous operations.
// Usage:
// async := NewAsync()
// async.Accept(func() {
//     fmt.Print("operation completed")
// }).Accept(func() {
////     fmt.Print("chained operation completed")
//// })
// go func() { async.Complete() }()
type Async struct {
	state 			uint32
	completions 	[]func()
	done       		chan struct{}
	sync.RWMutex
}

// NewAsync initializes a new async object.
func NewAsync() *Async {
	return &Async{ state:pending, completions: []func(){}, done:make(chan struct{}) }
}

// AsCompletedAsync creates an already completed async object.
func AsCompletedAsync() *Async {
	async := NewAsync()
	async.Complete()
	return async
}

// IsCompleted returns true if the async operation is marked as completed, otherwise false.
func (async *Async) IsCompleted() bool {
	state := atomic.LoadUint32(&async.state)
	return state == completed
}

// IsPending returns true if the async operation is running, otherwise false.
func (async *Async) IsPending() bool {
	state := atomic.LoadUint32(&async.state)
	return state == pending
}

// IsCancelled returns true if the async operation is cancelled, otherwise false.
func (async *Async) IsCancelled() bool {
	state := atomic.LoadUint32(&async.state)
	return state == cancelled
}

// Accept allows the chaining of the async operations after each other
// and subscribes a simple a callback function called when the async operation completed.
// For example:
// async.Accept(func() {
//     fmt.Print("operation completed")
// })
func (async *Async) Accept(completion func()) *Async {
	if async.IsCompleted() {
		completion()
	}

	if async.IsPending() {
		async.Lock()
		async.completions = append(async.completions, completion)
		async.Unlock()
	}

	return async
}

// Apply allows the chaining of the async operations after each other and subscribes a
// callback function which called when the async operation completed.
// Returns an AsyncResult object which returns a result.
// For example:
// async.Apply(func() {
//     return "new result"
// })
func (async *Async) Apply(completion func() interface{}) *AsyncResult {
	asyncResult := NewAsyncResult()
	async.Accept(func(){
		newResult := completion()
		asyncResult.Complete(newResult)
	})

	return asyncResult
}

// Complete moves the async operation into the completed state.
func (async *Async) Complete() {
	if atomic.CompareAndSwapUint32(&async.state, pending, completed) {
		close(async.done)
		async.RLock()
		defer async.RUnlock()
		for _, comp := range async.completions {
			comp()
		}
	}
	async.completions = nil
}

// Cancel prevents the calling of the completion handlers and
// the remaining chained operations to be invoked.
func (async *Async) Cancel() {
	if atomic.CompareAndSwapUint32(&async.state, pending, cancelled) {
		close(async.done)
	}
	async.completions = nil
}

// Wait blocks until the async operation is completed.
func (async *Async) Wait() {
	<-async.done
}

// Wait blocks until the async operation is completed or until
// the given timeout duration expires.
func (async *Async) WaitOrTimeout(duration time.Duration) (error) {
	timer:= time.NewTimer(duration)
	defer timer.Stop()

	select {
	case <-timer.C:
		return &CancelledError{}
	case <-async.done:
		return nil
	}
}
