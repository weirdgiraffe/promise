package promise

import "context"

type Function func() (result interface{}, err error)

type Promise struct {
	fn       Function
	async    AsyncExecutor
	callback []Callback

	done chan struct{}
	res  interface{}
	err  error
}

func New(fn Function, callback ...Callback) *Promise {
	return WithExecutor(DefaultExecutor, fn, callback...)
}

func WithExecutor(exec AsyncExecutor, fn Function, callback ...Callback) *Promise {
	pr := &Promise{
		fn:       fn,
		callback: callback,
		async:    exec,
		done:     make(chan struct{}),
	}
	pr.async.ExecAsync(pr)
	return pr
}

func (pr *Promise) Done() <-chan struct{} {
	return pr.done
}

func (pr *Promise) Result() (interface{}, error) {
	<-pr.Done()
	return pr.res, pr.err
}

func (pr *Promise) ResultWithContext(ctx context.Context) (interface{}, error) {
	select {
	case <-pr.Done():
		return pr.res, pr.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (pr *Promise) Cancel() {
	pr.Reject(context.Canceled)
}

func (pr *Promise) Resolve(v interface{}) {
	pr.finalize(v, nil)
}

func (pr *Promise) Reject(err error) {
	pr.finalize(nil, err)
}

func (pr *Promise) ExecFn() (interface{}, error) {
	return pr.fn()
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

type Callback interface {
	PromiseCallback(res interface{}, err error)
}

type OnDone func()

func (fn OnDone) PromiseCallback(_ interface{}, _ error) {
	fn()
}

type OnSuccess func(res interface{})

func (fn OnSuccess) PromiseCallback(res interface{}, err error) {
	if err == nil {
		fn(res)
	}
}

type OnError func(err error)

func (fn OnError) PromiseCallback(res interface{}, err error) {
	if err != nil {
		fn(err)
	}
}

type AsyncExecutor interface {
	ExecAsync(p *Promise)
}

type AsyncExecutorFunc func(p *Promise)

func (fn AsyncExecutorFunc) ExecAsync(pr *Promise) {
	fn(pr)
}

var DefaultExecutor = AsyncExecutorFunc(func(pr *Promise) {
	go func() {
		res, err := pr.ExecFn()
		if err != nil {
			pr.Reject(err)
			return
		}
		pr.Resolve(res)
	}()
})
