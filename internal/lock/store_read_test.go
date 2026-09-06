package lock

import (
	"errors"
	"io/fs"
	"runtime"
	"testing"
	"time"
)

func TestReadStoreWithRetryRetriesTransientThenDecodes(t *testing.T) {
	wantErr := &fs.PathError{Op: "open", Path: "store.json", Err: fs.ErrPermission}
	valid := []byte(`{"version":3}`)
	attempts := 0
	clock := time.Unix(100, 0)
	raw, err := readStoreWithRetry("store.json", func(string) ([]byte, error) {
		attempts++
		if attempts == 1 {
			return nil, wantErr
		}
		return valid, nil
	}, func() time.Time { return clock }, func(d time.Duration) { clock = clock.Add(d) }, func(err error) bool {
		return errors.Is(err, fs.ErrPermission)
	})
	if err != nil {
		t.Fatalf("readStoreWithRetry: %v", err)
	}
	store, err := decodeStore(raw, "store.json")
	if err != nil {
		t.Fatalf("decode retried bytes: %v", err)
	}
	if store.Version != 3 || attempts != 2 {
		t.Fatalf("decoded store version=%d after %d attempts, want version 3 after 2 attempts", store.Version, attempts)
	}
}

func TestReadStoreWithRetryDoesNotRetryNonTransient(t *testing.T) {
	wantErr := errors.New("permanent read failure")
	attempts := 0
	_, err := readStoreWithRetry("store.json", func(string) ([]byte, error) {
		attempts++
		return nil, wantErr
	}, time.Now, func(time.Duration) { t.Fatal("non-transient read was retried") }, func(error) bool { return false })
	if !errors.Is(err, wantErr) || attempts != 1 {
		t.Fatalf("error=%v attempts=%d, want original error after one attempt", err, attempts)
	}
}

func TestReadStoreWithRetryExhaustsTransientWithOriginalError(t *testing.T) {
	wantErr := errors.New("sharing violation")
	attempts := 0
	clock := time.Unix(200, 0)
	_, err := readStoreWithRetry("store.json", func(string) ([]byte, error) {
		attempts++
		return nil, wantErr
	}, func() time.Time { return clock }, func(d time.Duration) { clock = clock.Add(d) }, func(error) bool { return true })
	if !errors.Is(err, wantErr) || attempts < 2 {
		t.Fatalf("error=%v attempts=%d, want original error after retries", err, attempts)
	}
}

func TestLockReadIsTransientMatchesPlatform(t *testing.T) {
	permission := &fs.PathError{Op: "open", Path: "store.json", Err: fs.ErrPermission}
	want := runtime.GOOS == "windows"
	if got := lockReadIsTransient(permission); got != want {
		t.Fatalf("lockReadIsTransient(permission) = %v on %s, want %v", got, runtime.GOOS, want)
	}
	if lockReadIsTransient(&fs.PathError{Op: "open", Path: "store.json", Err: fs.ErrNotExist}) {
		t.Fatal("missing store was classified as a transient read failure")
	}
}
