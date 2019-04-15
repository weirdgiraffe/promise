package promise

import (
	"context"
	"errors"
	"testing"
	"time"
)

var expectedError = errors.New("some error")

func TestPromiseCallbacks(t *testing.T) {
	var tt = []struct {
		Name            string
		Fn              Function
		ExpectedValue   string
		ExpectedError   error
		ExpectOnSuccess bool
		ExpectOnError   bool
	}{
		{
			Name:            "success",
			Fn:              func() (interface{}, error) { return "hello world", nil },
			ExpectedValue:   "hello world",
			ExpectOnSuccess: true,
		},
		{
			Name:          "failure",
			Fn:            func() (interface{}, error) { return nil, expectedError },
			ExpectedError: expectedError,
			ExpectOnError: true,
		},
	}

	for _, tc := range tt {
		t.Run(tc.Name, func(t *testing.T) {
			var onSuccessCalled, onErrorCalled, onDoneCalled bool

			p := New(
				tc.Fn,
				OnSuccess(func(v interface{}) {
					onSuccessCalled = true
					if !tc.ExpectOnSuccess {
						t.Fatalf("unexpected OnSuccess callback with value %v", v)
					}
					s, ok := v.(string)
					if !ok {
						t.Logf("exp: string")
						t.Logf("got: %T", v)
						t.Fatalf("unexpected value type")
					}
					if tc.ExpectedValue != s {
						t.Logf("exp: %s", tc.ExpectedValue)
						t.Logf("got: %s", s)
						t.Errorf("unexpected value")
					}
				}),
				OnError(func(err error) {
					onErrorCalled = true
					if !tc.ExpectOnError {
						t.Fatalf("unexpected OnError callback with error %v", err)
					}
					if tc.ExpectedError != err {
						t.Logf("exp: %v", tc.ExpectedError)
						t.Logf("got: %v", err)
						t.Errorf("unexpected error")
					}
				}),
				OnDone(func() {
					onDoneCalled = true
				}),
			)

			select {
			case <-time.After(time.Second):
				t.Fatal("promise was not done within a second")
			case <-p.Done():
			}

			if tc.ExpectOnSuccess && !onSuccessCalled {
				t.Errorf("OnSuccess callback was not being called")
			}
			if tc.ExpectOnError && !onErrorCalled {
				t.Errorf("OnError callback was not being called")
			}
			if !onDoneCalled {
				t.Errorf("OnDone callback was not being called")
			}
		})
	}
}

func TestPromiseCancelation(t *testing.T) {
	pr := New(func() (interface{}, error) {
		time.Sleep(time.Second)
		return nil, nil
	})
	pr.Cancel()
	if _, err := pr.Result(); err != context.Canceled {
		t.Logf("exp: %T", context.Canceled)
		t.Logf("got: %T", err)
		t.Fatal("unexpected error type on cancel")
	}
}

func TestPromiseResultWithContext(t *testing.T) {
	expectedValue := "hello world"
	pr := New(func() (interface{}, error) {
		time.Sleep(100 * time.Millisecond)
		return expectedValue, nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	v, err := pr.ResultWithContext(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %[1]v (%[1]T)", err)
	}

	s, ok := v.(string)
	if !ok {
		t.Logf("exp: string")
		t.Logf("got: %T", v)
		t.Fatalf("unexpected value type")
	}
	if expectedValue != s {
		t.Logf("exp: %s", expectedValue)
		t.Logf("got: %s", s)
		t.Errorf("unexpected value")
	}
}

func TestPromiseResultWithContextContinueExecutionIfContextCanceled(t *testing.T) {
	expectedValue := "hello world"
	pr := New(func() (interface{}, error) {
		time.Sleep(200 * time.Millisecond)
		return expectedValue, nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	if _, err := pr.ResultWithContext(ctx); err != context.DeadlineExceeded {
		t.Logf("exp: %T", context.Canceled)
		t.Logf("got: %T", err)
		t.Fatalf("unexpected error type on context timeout")
	}

	v, err := pr.Result()
	if err != nil {
		t.Fatalf("unexpected error: %[1]v (%[1]T)", err)
	}

	s, ok := v.(string)
	if !ok {
		t.Logf("exp: string")
		t.Logf("got: %T", v)
		t.Fatalf("unexpected value type")
	}
	if expectedValue != s {
		t.Logf("exp: %s", expectedValue)
		t.Logf("got: %s", s)
		t.Errorf("unexpected value")
	}
}

func TestDoubleResolveFromExecutor(t *testing.T) {
	expectedValue := "hello alice"
	unexpectedValue := "hello bob"

	ex := AsyncExecutorFunc(func(pr *Promise) {
		pr.Resolve(expectedValue)
		pr.Resolve(unexpectedValue)
	})

	pr := WithExecutor(ex, func() (interface{}, error) { return nil, nil })
	v, err := pr.Result()
	if err != nil {
		t.Fatalf("unexpected error: %[1]v (%[1]T)", err)
	}

	s, ok := v.(string)
	if !ok {
		t.Logf("exp: string")
		t.Logf("got: %T", v)
		t.Fatalf("unexpected value type")
	}
	if expectedValue != s {
		t.Logf("exp: %s", expectedValue)
		t.Logf("got: %s", s)
		t.Errorf("unexpected value")
	}
}

func TestDoubleRejectFromExecutor(t *testing.T) {
	unexpectedError := errors.New("unexpected error")

	ex := AsyncExecutorFunc(func(pr *Promise) {
		pr.Reject(expectedError)
		pr.Reject(unexpectedError)
	})

	pr := WithExecutor(ex, func() (interface{}, error) { return nil, nil })

	_, err := pr.Result()
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if expectedError != err {
		t.Logf("exp: %v", expectedError)
		t.Logf("got: %v", err)
		t.Errorf("unexpected error")
	}
}
