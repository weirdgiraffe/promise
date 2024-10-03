package promise

import (
	"errors"
	"sync"
	"time"
)

const (
	DefaultRunnerConcurrency = 4
	DefaultRunnerCapacity    = 100
)

var (
	ErrCanceled      = errors.New("promise canceled")
	ErrExecutionDone = errors.New("execution is done")
)

type Promise[T any] struct {
	done   chan struct{}
	result T
	err    error
}

func NewPromise[T any]() *Promise[T] {
	return &Promise[T]{
		done: make(chan struct{}),
	}
}

// Done chanel which will be closed if promise is either resolved or rejected
func (p *Promise[T]) Done() <-chan struct{} {
	return p.done
}

// Result returns result of the undelying promis. This method will block until
// promise is either resolved or rejected.
func (p *Promise[T]) Result() (T, error) {
	<-p.Done()
	return p.result, p.err
}

// Cancel cancel the promise, by failing it with ErrCanceled
func (p *Promise[T]) Cancel() {
	p.Reject(ErrCanceled)
}

// Canceled returns true if promise was canceled
func (p *Promise[T]) Canceled() bool {
	select {
	case <-p.Done():
		return p.err == ErrCanceled
	default:
		return false
	}
}

// Reject rejects the promise with err
func (p *Promise[T]) Reject(err error) {
	select {
	case <-p.Done():
		// if promise is already done, then its to late
		return
	default:
		p.err = err
		close(p.done)
	}
}

// Resolve resolves the promise with v
func (p *Promise[T]) Resolve(v T) {
	select {
	case <-p.Done():
		// if promise is already done, then its to late
		return
	default:
		p.result = v
		close(p.done)
	}
}

type Rejectable interface {
	Reject(error)
}

// RejectAll rejects all promises
func RejectAll[S []E, E Rejectable](err error, promises S) {
	for i := range promises {
		promises[i].Reject(err)
	}
}

type execPromise struct {
	promise Rejectable
	exec    func()
}

type Runner struct {
	stoping  chan struct{}
	promises chan execPromise
	wg       sync.WaitGroup
}

func NewRunner(conc int, capacity int) *Runner {
	r := &Runner{
		stoping:  make(chan struct{}),
		promises: make(chan execPromise, capacity),
		wg:       sync.WaitGroup{},
	}
	r.wg.Add(conc)
	for i := 0; i < conc; i++ {
		go func() {
			defer r.wg.Done()
			for {
				select {
				case <-r.stoping:
					return
				default:
				}

				select {
				case <-r.stoping:
					return
				case promise := <-r.promises:
					promise.exec()
				}
			}
		}()
	}

	return r
}

// Wait executes all pending promises and stops
// execution of all further promises
func (r *Runner) Wait() {
	select {
	case <-r.stoping:
		return
	default:
		close(r.stoping)
	}
	r.wg.Wait()

	// wait for at most 1ms to finish all pending Async calls
	// which are already writting to promises channel.
	closed := make(chan struct{})
	go func() {
		defer close(closed)
		defer close(r.promises)
		for {
			select {
			case item := <-r.promises:
				item.exec()
			case <-time.After(time.Millisecond):
				return
			}
		}
	}()
	<-closed
	for item := range r.promises {
		item.exec()
	}
}

var DefaultRunner = NewRunner(DefaultRunnerConcurrency, DefaultRunnerCapacity)

func Async[T any](impl func() (T, error)) *Promise[T] {
	return AsyncOnRunner(DefaultRunner, impl)
}

func Wait() {
	DefaultRunner.Wait()
}

func AsyncOnRunner[T any](r *Runner, impl func() (T, error)) *Promise[T] {
	promise := NewPromise[T]()
	item := execPromise{
		promise: promise,
		exec: func() {
			result, err := impl()
			if err != nil {
				promise.Reject(err)
			} else {
				promise.Resolve(result)
			}
		},
	}

	// select picks channel randomly, so prioritize r.done
	// channel to avoid writing to the closed promise channel
	select {
	case <-r.stoping:
		promise.Reject(ErrExecutionDone)
		return promise
	default:
	}

	select {
	case <-r.stoping:
		promise.Reject(ErrExecutionDone)
	case r.promises <- item:
	}

	return promise
}
