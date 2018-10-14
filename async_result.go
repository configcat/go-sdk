package configcat

import (
	"sync"
	"sync/atomic"
	"time"
)

// AsyncResult describes an object which used to control asynchronous operations with return value.
// Allows the chaining of these operations after each other.
// Usage:
// async := NewAsync()
// async.ApplyThen(func(result interface{}) {
// 	   fmt.Print(result)
//     return "new result"
// }).Apply(func(previousResult interface{}) {
//     fmt.Print("chained operation completed")
// })
// go func() { async.Complete("success") }()
type AsyncResult struct {
	state       uint32
	completions []func(result interface{})
	done        chan struct{}
	result      interface{}
	*Async
	sync.RWMutex
}

// NewAsyncResult initializes a new async object with result.
func NewAsyncResult() *AsyncResult {
	return &AsyncResult{state: pending, completions: []func(result interface{}){}, done: make(chan struct{}), Async: NewAsync()}
}

// AsCompletedAsyncResult creates an already completed async object.
func AsCompletedAsyncResult(result interface{}) *AsyncResult {
	async := NewAsyncResult()
	async.Complete(result)
	return async
}

// Apply allows the chaining of the async operations after each other and subscribes a
// callback function which gets the operation result as argument and called when the async
// operation completed. Returns an AsyncResult object. For example:
// async.Apply(func(result interface{}) {
//     fmt.Print(result)
// })
func (asyncResult *AsyncResult) Apply(completion func(result interface{})) *AsyncResult {
	if asyncResult.IsCompleted() {
		completion(asyncResult.result)
	}

	if asyncResult.IsPending() {
		asyncResult.Lock()
		asyncResult.completions = append(asyncResult.completions, completion)
		asyncResult.Unlock()
	}

	return asyncResult
}

// Accept allows the chaining of the async operations after each other and subscribes a
// callback function which gets the operation result as argument and called when the async
// operation completed. Returns an Async object. For example:
// async.Accept(func(result interface{}) {
//     fmt.Print(result)
// })
func (asyncResult *AsyncResult) Accept(completion func(result interface{})) *Async {
	return asyncResult.Async.Accept(func() {
		completion(asyncResult.result)
	})
}

// ApplyThen allows the chaining of the async operations after each other and subscribes a
// callback function which gets the operation result as argument and called when the async
// operation completed. Returns an AsyncResult object which returns a different result type.
// For example:
// async.Accept(func(result interface{}) {
//     fmt.Print(result)
// })
func (asyncResult *AsyncResult) ApplyThen(completion func(result interface{}) interface{}) *AsyncResult {
	newAsyncResult := NewAsyncResult()
	asyncResult.Accept(func(result interface{}) {
		newResult := completion(result)
		newAsyncResult.Complete(newResult)
	})
	return newAsyncResult
}

// Complete moves the async operation into the completed state.
// Gets the result of the operation as argument.
func (asyncResult *AsyncResult) Complete(result interface{}) {
	if atomic.CompareAndSwapUint32(&asyncResult.state, pending, completed) {
		asyncResult.result = result
		asyncResult.Async.Complete()
		close(asyncResult.done)
		asyncResult.RLock()
		defer asyncResult.RUnlock()
		for _, comp := range asyncResult.completions {
			comp(result)
		}
	}
	asyncResult.completions = nil
}

// Cancel prevents the calling of the completion handlers and
// the remaining chained operations to be invoked.
func (asyncResult *AsyncResult) Cancel() {
	if atomic.CompareAndSwapUint32(&asyncResult.state, pending, cancelled) {
		asyncResult.Async.Cancel()
		close(asyncResult.done)
	}
	asyncResult.completions = nil
}

// Get blocks until the async operation is completed,
// then returns the result of the operation.
func (asyncResult *AsyncResult) Get() interface{} {
	<-asyncResult.done
	return asyncResult.result
}

// GetOrTimeout blocks until the async operation is completed or until
// the given timeout duration expires, then returns the result of the operation.
func (asyncResult *AsyncResult) GetOrTimeout(duration time.Duration) (interface{}, error) {
	timer := time.NewTimer(duration)
	defer timer.Stop()

	select {
	case <-timer.C:
		return nil, &CancelledError{}
	case <-asyncResult.done:
		return asyncResult.result, nil
	}
}
