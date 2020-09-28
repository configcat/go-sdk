package configcat

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

// async describes an object which used to control asynchronous operations.
// Usage:
//  async := newAsync()
//  async.accept(func() {
//     fmt.Print("operation completed")
//  }).accept(func() {
//     fmt.Print("chained operation completed")
//  })
//  go func() { async.complete() }()
type async struct {
	state       uint32
	completions []func()
	done        chan struct{}
	sync.RWMutex
}

// newAsync initializes a new async object.
func newAsync() *async {
	return &async{state: pending, completions: []func(){}, done: make(chan struct{})}
}

// isCompleted returns true if the async operation is marked as completed, otherwise false.
func (async *async) isCompleted() bool {
	state := atomic.LoadUint32(&async.state)
	return state == completed
}

// isPending returns true if the async operation is running, otherwise false.
func (async *async) isPending() bool {
	state := atomic.LoadUint32(&async.state)
	return state == pending
}

// accept allows the chaining of the async operations after each other
// and subscribes a simple a callback function called when the async operation completed.
// For example:
//  async.accept(func() {
//     fmt.Print("operation completed")
//  })
func (async *async) accept(completion func()) *async {
	if async.isCompleted() {
		completion()
	}

	if async.isPending() {
		async.Lock()
		async.completions = append(async.completions, completion)
		async.Unlock()
	}

	return async
}

// apply allows the chaining of the async operations after each other and subscribes a
// callback function which called when the async operation completed.
// Returns an asyncResult object which returns a result.
// For example:
//  async.apply(func() {
//     return "new result"
//  })
func (async *async) apply(completion func() interface{}) *asyncResult {
	asyncResult := newAsyncResult()
	async.accept(func() {
		newResult := completion()
		asyncResult.complete(newResult)
	})

	return asyncResult
}

// complete moves the async operation into the completed state.
func (async *async) complete() {
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

// wait blocks until the async operation is completed.
func (async *async) wait() {
	<-async.done
}

// waitOrTimeout blocks until the async operation is completed or until
// the given timeout duration expires.
func (async *async) waitOrTimeout(duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-timer.C:
		return errors.New("operation cancelled")
	case <-async.done:
		return nil
	}
}
