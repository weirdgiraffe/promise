package promise

import "context"

// Promise structure which defines a promise
type Promise struct {
	callback []Callback

	done chan struct{}
	res  interface{}
	err  error
}

type PromiseFunc func() (interface{}, error)

// Function creates a promise for function execution
func Execution(fn PromiseFunc, cb ...Callback) *Promise {
	return DefaultExecutor.PromiseAsyncExecution(fn, cb...)
}

// New intializes and runs the promise for function fn with callbacks. This promise will be ran on DefaultExecutor
func New(cb ...Callback) *Promise {
	return &Promise{
		callback: cb,
		done:     make(chan struct{}),
	}
}

// Done chanel which will be close if promise is either resolved or rejected
func (pr *Promise) Done() <-chan struct{} {
	return pr.done
}

// Result will block until promise is either resolved or rejected, then returns promise result and error
func (pr *Promise) Result() (interface{}, error) {
	<-pr.Done()
	return pr.res, pr.err
}

// ResultWithContext will block until promise is either resolved or rejected or until context is done.
// In case of context is done returns the error from context, otherwise returns promise's result and error.
// NOTE: if context is done promise will continue it's execution
func (pr *Promise) ResultWithContext(ctx context.Context) (interface{}, error) {
	select {
	case <-pr.Done():
		return pr.res, pr.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Cancel cancel the promise, by failing it with context.Canceled
func (pr *Promise) Cancel() {
	pr.Reject(context.Canceled)
}

// Resolve resolves the promise, calls OnSuccess, OnDone and other suitable callbacks
func (pr *Promise) Resolve(v interface{}) {
	pr.finalize(v, nil)
}

// Reject reject the promise, calls OnError, OnDone and other suitable callbacks
func (pr *Promise) Reject(err error) {
	pr.finalize(nil, err)
}

func (pr *Promise) finalize(v interface{}, err error) {
	select {
	case <-pr.done:
		return
	default:
		pr.res = v
		pr.err = err
		close(pr.done)
	}
	for _, cb := range pr.callback {
		cb.PromiseCallback(v, err)
	}
}

// Callback defines a promise's callback.
type Callback interface {
	// PromiseCallback will be called when promise is either resolved or rejected.
	PromiseCallback(res interface{}, err error)
}

// OnDone promise's callback. Will be called when promise is either resolved or rejected.
type OnDone func()

// PromiseCallback implements Callback interface
func (fn OnDone) PromiseCallback(_ interface{}, _ error) {
	fn()
}

// OnSuccess promise's callback. Will be called when promise is resolved.
type OnSuccess func(res interface{})

// PromiseCallback implements Callback interface
func (fn OnSuccess) PromiseCallback(res interface{}, err error) {
	if err == nil {
		fn(res)
	}
}

// OnError promise's callback. Will be called when promise is rejected.
type OnError func(err error)

// PromiseCallback implements Callback interface
func (fn OnError) PromiseCallback(res interface{}, err error) {
	if err != nil {
		fn(err)
	}
}

// AsyncExecutor is an interface you should implement if you want to define
// a custom promises executor
type AsyncExecutor interface {
	PromiseAsyncExecution(fn PromiseFunc, cb ...Callback) *Promise
}

// AsyncExecutorFunc is a function pattern applied on top of AsyncExecutor
type AsyncExecutorFunc func(PromiseFunc, []Callback) *Promise

// ExecAsync implements AsyncExecutor interface
func (fn AsyncExecutorFunc) PromiseAsyncExecution(promiseFn PromiseFunc, cb ...Callback) *Promise {
	return fn(promiseFn, cb)
}

// DefaultExecutor executes each promise in a separate go routine
var DefaultExecutor = AsyncExecutorFunc(func(fn PromiseFunc, cb []Callback) *Promise {
	pr := New(cb...)
	go func() {
		res, err := fn()
		if err != nil {
			pr.Reject(err)
			return
		}
		pr.Resolve(res)
	}()
	return pr
})

// WhenAll return the list of promises results corresponding to the promises list p
// if any promise in p failes - fails with that error.
// NOTE: This function doesn't cancel rest of the promises in p on error.
func WhenAll(p ...*Promise) *Promise {
	np := Execution(func() (interface{}, error) {
		l := make([]interface{}, len(p))
		for i := range p {
			res, err := p[i].Result()
			if err != nil {
				return nil, err
			}
			l[i] = res
		}
		return l, nil
	})

	return np
}

// WhenAny return the first result for the list op promises p. If all promises in
// the promises list p were failed - returns the error of last failed promise.
func WhenAny(p ...*Promise) *Promise {
	np := Execution(func() (interface{}, error) {
		var res interface{}
		var err error
		for i := range p {
			res, err = p[i].Result()
			if err == nil {
				return res, nil
			}
		}
		return nil, err
	})
	return np
}

// CancelAll cancel all promises
func CancelAll(p ...*Promise) {
	for i := range p {
		p[i].Cancel()
	}
}
