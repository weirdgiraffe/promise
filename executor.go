package promise

import (
	"errors"
	"sync"
)

var ErrExecutorStopped = errors.New("executor is stopped")

type PromiseFunc func() (interface{}, error)

type executionPromise struct {
	*Promise
	fn PromiseFunc
}

type Executor struct {
	stopCh chan struct{}
	promCh chan *executionPromise
	wg     sync.WaitGroup
}

func StartExecutor(concurrency int, maxPendingPromises int) *Executor {
	e := &Executor{
		stopCh: make(chan struct{}),
		promCh: make(chan *executionPromise, maxPendingPromises),
	}

	for i := 0; i < concurrency; i++ {
		e.wg.Add(1)
		go func() {
			defer e.wg.Done()
			for {
				select {
				case <-e.stopCh:
					return
				default:
				}

				select {
				case <-e.stopCh:
					return
				case p := <-e.promCh:
					res, err := p.fn()
					if err != nil {
						p.Reject(err)
					} else {
						p.Resolve(res)
					}
				}
			}
		}()
	}

	return e
}

// Cap return the maximum amount of promises this Executor can handle
// until it blocks
func (e *Executor) Cap() int {
	return cap(e.promCh)
}

func (e *Executor) Stop() {
	select {
	case <-e.stopCh:
		return
	default:
	}

	close(e.stopCh)
	e.wg.Wait()

	// cancel all pending promises
	for {
		select {
		case ep := <-e.promCh:
			ep.Reject(ErrExecutorStopped)
		default:
			return
		}
	}
}

func (e *Executor) Exec(fn PromiseFunc) *Promise {
	ep := &executionPromise{
		Promise: New(),
		fn:      fn,
	}
	// Try to send it to promises channel
	select {
	case <-e.stopCh:
		ep.Reject(ErrExecutorStopped)
		return ep.Promise
	default:
	}

	select {
	case <-e.stopCh:
		ep.Reject(ErrExecutorStopped)
	case e.promCh <- ep:
	}
	return ep.Promise
}

// WhenAll return the list of promises results corresponding to the promises list p
// if any promise in p failes - fails with that error.
// NOTE: This function doesn't cancel rest of the promises in p on error.
func WhenAll(e *Executor, p ...*Promise) *Promise {
	np := e.Exec(func() (interface{}, error) {
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
func WhenAny(e *Executor, p ...*Promise) *Promise {
	np := e.Exec(func() (interface{}, error) {
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
