package promise

import (
	"context"
	"errors"
)

var ErrCanceled = errors.New("promise canceled")

// Promise structure which defines a promise
type Promise struct {
	done chan struct{}
	res  interface{}
	err  error
}

// New intializes the promise which must be resolved or rejected later
func New() *Promise {
	return &Promise{
		done: make(chan struct{}),
	}
}

// Done chanel which will be closed if promise is either resolved or rejected
func (p *Promise) Done() <-chan struct{} {
	return p.done
}

// Result returns result of the undelying promis. This method will block until
// promise is either resolved or rejected.
func (p *Promise) Result() (interface{}, error) {
	<-p.Done()
	return p.res, p.err
}

// ResultWithContext returns the result of the undelying promise. This method
// blocks until context is canceled or promise is done.
func (p *Promise) ResultWithContext(ctx context.Context) (interface{}, error) {
	select {
	case <-p.Done():
		return p.res, p.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Cancel cancel the promise, by failing it with ErrCanceled
func (p *Promise) Cancel() {
	p.Reject(ErrCanceled)
}

// Canceled returns true if promise was canceled
func (p *Promise) Canceled() bool {
	select {
	case <-p.Done():
		return p.err == ErrCanceled
	default:
		return false
	}
}

// Resolve resolves the promise with v
func (p *Promise) Resolve(v interface{}) {
	p.finalize(v, nil)
}

// Reject rejects the promise with err
func (p *Promise) Reject(err error) {
	p.finalize(nil, err)
}

func (p *Promise) finalize(v interface{}, err error) {
	select {
	case <-p.Done():
		// ignore all finalizations but the first one
		return
	default:
		p.res = v
		p.err = err
		close(p.done)
	}
}

// CancelAll cancel all promises
func CancelAll(p ...*Promise) {
	for i := range p {
		p[i].Cancel()
	}
}
