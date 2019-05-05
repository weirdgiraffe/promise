package promise

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestPromiseIsNotDoneAfterCreation(t *testing.T) {
	p := New()

	select {
	case <-p.Done():
		t.Fatalf("promise is done before being resolved or rejected")
	default:
	}
}

func TestPromiseCouldBeResolved(t *testing.T) {
	expected := "hello world"

	p := New()
	p.Resolve(expected)

	select {
	case <-p.Done():
		v, err := p.Result()
		if err != nil {
			t.Fatalf("expected nil, got error %[1]v (%[1]T)", err)
		}
		if v.(string) != expected {
			t.Logf("exp: %s", expected)
			t.Logf("got: %s", v.(string))
			t.Fatalf("unexpected result")
		}
	default:
		t.Fatalf("promise is not done after being resolved")
	}
}

func TestPromiseCouldBeRejected(t *testing.T) {
	expectedError := errors.New("hello world")

	p := New()
	p.Reject(expectedError)

	select {
	case <-p.Done():
		v, err := p.Result()
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
		if v != nil {
			t.Fatalf("unexpected promise result: %v", v)
		}
		if err != expectedError {
			t.Logf("exp: %v", expectedError)
			t.Logf("got: %v", err)
			t.Fatalf("unexpected error")
		}
	default:
		t.Fatalf("promise is not done after being rejected")
	}
}

func TestPromiseCancelation(t *testing.T) {
	p := New()

	if p.Canceled() {
		t.Fatalf("promise is canceled after creation")
	}

	p.Cancel()

	if !p.Canceled() {
		t.Fatalf("promise was not canceled")
	}
}

func TestPromiseResultWithContextIfContextCanceled(t *testing.T) {
	p := New()

	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()

	_, err := p.ResultWithContext(ctx)

	if err != context.DeadlineExceeded {
		t.Logf("exp: %v", context.DeadlineExceeded)
		t.Logf("got: %v", err)
		t.Fatalf("unexpected error")
	}
}

func TestPromiseResultWithContextIfPromiseIsDone(t *testing.T) {
	expectedValue := "hello world"
	p := New()

	go func() {
		time.Sleep(100 * time.Millisecond)
		p.Resolve(expectedValue)
	}()

	v, err := p.ResultWithContext(context.Background())

	if err != nil {
		t.Fatalf("unexpected error %[1]v (%[1]T)", err)
	}

	if v.(string) != expectedValue {
		t.Logf("exp: %v", expectedValue)
		t.Logf("got: %v", v.(string))
		t.Fatalf("unexpected promise value")
	}
}

func TestPromiseDoubleResolve(t *testing.T) {
	expectedValue := "hello alice"
	unexpectedValue := "hello bob"

	p := New()
	p.Resolve(expectedValue)
	p.Resolve(unexpectedValue)

	v, err := p.Result()
	if err != nil {
		t.Fatalf("unexpected error %[1]v (%[1]T)", err)
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

func TestDoubleReject(t *testing.T) {
	expectedError := errors.New("hello world")
	unexpectedError := errors.New("unexpected error")

	p := New()
	p.Reject(expectedError)
	p.Reject(unexpectedError)

	_, err := p.Result()
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if expectedError != err {
		t.Logf("exp: %v", expectedError)
		t.Logf("got: %v", err)
		t.Errorf("unexpected error")
	}
}

func TestCancelAll(t *testing.T) {
	l := make([]*Promise, 5)
	for i := range l {
		l[i] = New()
	}

	CancelAll(l...)

	for i := range l {
		if _, err := l[i].Result(); err != ErrCanceled {
			t.Logf("exp: %[1]T %[1]v", ErrCanceled)
			t.Logf("got: %[1]T %[1]v", err)
			t.Error("unexpected error of canceled promise")
		}
	}
}
