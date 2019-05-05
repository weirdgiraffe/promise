package promise

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestPromiseExecutorExecResolve(t *testing.T) {
	e := StartExecutor(4, 100)
	defer e.Stop()

	expectedValue := "hello world"

	p := e.Exec(func() (interface{}, error) {
		return expectedValue, nil
	})

	v, err := p.Result()
	if err != nil {
		t.Fatalf("unexpected error %[1]v (%[1]T)", err)
	}

	if v.(string) != expectedValue {
		t.Logf("exp: %s", expectedValue)
		t.Logf("got: %s", v.(string))
		t.Error("unexpected promise value")
	}
}

func TestPromiseExecutorExecReject(t *testing.T) {
	e := StartExecutor(4, 100)
	defer e.Stop()

	expectedError := errors.New("hello world")

	p := e.Exec(func() (interface{}, error) {
		return nil, expectedError
	})

	_, err := p.Result()
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	if err != expectedError {
		t.Logf("exp: %v", expectedError)
		t.Logf("got: %v", err)
		t.Error("unexpected promise error")
	}
}

func TestPromiseExecutorExecOnStopedExecutor(t *testing.T) {
	e := StartExecutor(1, 1)

	expectedValue := "hello world"
	running := e.Exec(func() (interface{}, error) {
		time.Sleep(200 * time.Millisecond)
		return expectedValue, nil
	})

	buffered := e.Exec(func() (interface{}, error) {
		return "", nil
	})

	var pending *Promise
	pendingPlanned := make(chan struct{})
	go func() {
		close(pendingPlanned)
		pending = e.Exec(func() (interface{}, error) {
			return "", errors.New("unexpected error")
		})
		_, err := pending.Result()
		if err != ErrExecutorStopped {
			t.Logf("exp: %[1]T %[1]v", ErrExecutorStopped)
			t.Logf("got: %[1]T %[1]v", err)
			t.Error("unexpected error of pending promise")
		}
	}()

	// runnin promise function will start that will block executor
	// because it has concurrency == 1.  Then pending promise function
	// will be placed into executor and wait for execution. And then
	// before longRunning promise is done we stop the executor.
	<-pendingPlanned
	e.Stop()

	rejected := e.Exec(func() (interface{}, error) {
		return "", nil
	})

	_, err := buffered.Result()
	if err != ErrExecutorStopped {
		t.Logf("exp: %[1]T %[1]v", ErrExecutorStopped)
		t.Logf("got: %[1]T %[1]v", err)
		t.Error("unexpected error of buffered promise")
	}

	_, err = rejected.Result()
	if err != ErrExecutorStopped {
		t.Logf("exp: %[1]T %[1]v", ErrExecutorStopped)
		t.Logf("got: %[1]T %[1]v", err)
		t.Error("unexpected error of rejected promise")
	}

	v, err := running.Result()
	if err != nil {
		t.Fatalf("unexpected error %[1]v (%[1]T)", err)
	}
	if v.(string) != expectedValue {
		t.Logf("exp: %s", expectedValue)
		t.Logf("got: %v", v)
		t.Error("unexpected value of long running promise")
	}
}

func TestWhenAllOk(t *testing.T) {
	e := StartExecutor(4, 100)
	defer e.Stop()

	expexted := []interface{}{
		"hello",
		"world",
	}
	p1 := e.Exec(func() (interface{}, error) {
		time.Sleep(100 * time.Millisecond)
		return "hello", nil
	})
	p2 := e.Exec(func() (interface{}, error) {
		time.Sleep(100 * time.Millisecond)
		return "world", nil
	})

	res, err := WhenAll(e, p1, p2).Result()
	if err != nil {
		t.Fatalf("unexpected error: %[1]v (%[1]T)", err)
	}

	s1, _ := json.Marshal(expexted)
	s2, _ := json.Marshal(res)
	if !bytes.Equal(s1, s2) {
		t.Logf("exp: %s", s1)
		t.Logf("got: %s", s2)
		t.Errorf("unexpected result")
	}
}

func TestWhenAllErr(t *testing.T) {
	e := StartExecutor(4, 100)
	defer e.Stop()

	expectedError := errors.New("hello world")
	p1 := e.Exec(func() (interface{}, error) {
		time.Sleep(100 * time.Millisecond)
		return "", expectedError
	})
	p2 := e.Exec(func() (interface{}, error) {
		time.Sleep(100 * time.Millisecond)
		return "world", nil
	})

	_, err := WhenAll(e, p1, p2).Result()
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if expectedError != err {
		t.Logf("exp: %v", expectedError)
		t.Logf("got: %v", err)
		t.Errorf("unexpected error")
	}
}

func TestWhenAnyOk(t *testing.T) {
	e := StartExecutor(4, 100)
	defer e.Stop()

	expectedError := errors.New("hello world")
	p1 := e.Exec(func() (interface{}, error) {
		time.Sleep(100 * time.Millisecond)
		return "", expectedError
	})
	expectedValue := "hello world"
	p2 := e.Exec(func() (interface{}, error) {
		time.Sleep(100 * time.Millisecond)
		return expectedValue, nil
	})

	res, err := WhenAny(e, p1, p2).Result()
	if err != nil {
		t.Fatalf("unexpected error: %[1]v (%[1]T)", err)
	}

	s1, _ := json.Marshal(expectedValue)
	s2, _ := json.Marshal(res)
	if !bytes.Equal(s1, s2) {
		t.Logf("exp: %s", s1)
		t.Logf("got: %s", s2)
		t.Errorf("unexpected when any result")
	}
}

func TestWhenAnyTakesLastError(t *testing.T) {
	e := StartExecutor(4, 100)
	defer e.Stop()

	p1 := e.Exec(func() (interface{}, error) {
		time.Sleep(100 * time.Millisecond)
		return "", errors.New("unexpected error")
	})
	expectedError := errors.New("hello world")
	p2 := e.Exec(func() (interface{}, error) {
		time.Sleep(100 * time.Millisecond)
		return "", expectedError
	})

	_, err := WhenAny(e, p1, p2).Result()
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if err != expectedError {
		t.Logf("exp: %v", expectedError)
		t.Logf("got: %v", err)
		t.Errorf("unexpected error")
	}
}
