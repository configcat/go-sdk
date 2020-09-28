package configcat

import (
	"errors"
	"time"
)

// AsyncResult describes an object which used to control asynchronous operations with return value.
// Allows the chaining of these operations after each other.
// Usage:
//  async := NewAsync()
//  async.ApplyThen(func(result interface{}) {
// 	   fmt.Print(result)
//     return "new result"
//  }).Apply(func(previousResult interface{}) {
//     fmt.Print("chained operation completed")
//  })
//  go func() { async.Complete("success") }()
type asyncResult struct {
	result      interface{}
	*async
}

// newAsyncResult initializes a new async object with result.
func newAsyncResult() *asyncResult {
	return &asyncResult{async: newAsync()}
}

// asCompletedAsyncResult creates an already completed async object.
func asCompletedAsyncResult(result interface{}) *asyncResult {
	async := newAsyncResult()
	async.complete(result)
	return async
}

// accept allows the chaining of the async operations after each other and subscribes a
// callback function which gets the operation result as argument and called when the async
// operation completed. Returns an Async object. For example:
//  async.accept(func(result interface{}) {
//     fmt.Print(result)
//  })
func (asyncResult *asyncResult) accept(completion func(result interface{})) *async {
	return asyncResult.async.accept(func() {
		completion(asyncResult.result)
	})
}

// applyThen allows the chaining of the async operations after each other and subscribes a
// callback function which gets the operation result as argument and called when the async
// operation completed. Returns an AsyncResult object which returns a different result type.
// For example:
//  async.accept(func(result interface{}) {
//     fmt.Print(result)
//  })
func (asyncResult *asyncResult) applyThen(completion func(result interface{}) interface{}) *asyncResult {
	newAsyncResult := newAsyncResult()
	asyncResult.accept(func(result interface{}) {
		newResult := completion(result)
		newAsyncResult.complete(newResult)
	})
	return newAsyncResult
}

// compose allows the chaining of the async operations after each other and subscribes a
// callback function which gets the operation result as argument and returns a new async object.
// Returns an AsyncResult object which returns a different result type.
// For example:
//  async.compose(func(result interface{}) {
//     newAsyncResult := newAsyncResult()
//
//     DoSomethingAsynchronously(func(result interface{}) {
//         newAsyncResult.complete(result)
//     }))
//
//     return newAsyncResult
//  })
func (asyncResult *asyncResult) compose(completion func(result interface{}) *asyncResult) *asyncResult {
	newAsyncResult := newAsyncResult()
	asyncResult.accept(func(result interface{}) {
		newResult := completion(result)
		newResult.accept(func(result interface{}) {
			newAsyncResult.complete(result)
		})
	})
	return newAsyncResult
}

// complete moves the async operation into the completed state.
// Gets the result of the operation as argument.
func (asyncResult *asyncResult) complete(result interface{}) {
	asyncResult.result = result
	asyncResult.async.complete()
}

// get blocks until the async operation is completed,
// then returns the result of the operation.
func (asyncResult *asyncResult) get() interface{} {
	<-asyncResult.done
	return asyncResult.result
}

// GetOrTimeout blocks until the async operation is completed or until
// the given timeout duration expires, then returns the result of the operation.
func (asyncResult *asyncResult) getOrTimeout(duration time.Duration) (interface{}, error) {
	timer := time.NewTimer(duration)
	defer timer.Stop()

	select {
	case <-timer.C:
		return nil, errors.New("operation cancelled")
	case <-asyncResult.done:
		return asyncResult.result, nil
	}
}
