package session

import (
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestManager_Save(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "session_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	mgr, err := NewManager(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	sess, err := mgr.GetOrCreate("test-session")
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	sess.AddMessage(Message{
		Role:      "user",
		Content:   "Hello",
		Timestamp: time.Now(),
	})

	if err := mgr.Save(sess); err != nil {
		t.Fatalf("Failed to save session: %v", err)
	}

	filePath := filepath.Join(tmpDir, "test-session.jsonl")
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Error("Session file was not created")
	}
}

func TestManager_ConcurrentSave_SameSession(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "session_concurrent_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	mgr, err := NewManager(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	sess, err := mgr.GetOrCreate("concurrent-test-session")
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	numGoroutines := 20
	numSavesPerGoroutine := 10

	var wg sync.WaitGroup
	var errorCount int64

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < numSavesPerGoroutine; j++ {
				sess.AddMessage(Message{
					Role:      "user",
					Content:   "Message from goroutine",
					Timestamp: time.Now(),
				})

				if err := mgr.Save(sess); err != nil {
					t.Errorf("Save failed in goroutine %d, iteration %d: %v", goroutineID, j, err)
					atomic.AddInt64(&errorCount, 1)
				}
			}
		}(i)
	}

	wg.Wait()

	if errorCount > 0 {
		t.Errorf("Total save errors: %d", errorCount)
	}

	loaded, err := mgr.load("concurrent-test-session")
	if err != nil {
		t.Fatalf("Failed to load session: %v", err)
	}

	expectedMessages := numGoroutines * numSavesPerGoroutine
	if len(loaded.Messages) != expectedMessages {
		t.Errorf("Expected %d messages, got %d", expectedMessages, len(loaded.Messages))
	}
}

func TestManager_ConcurrentSave_DifferentSessions(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "session_multi_concurrent_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	mgr, err := NewManager(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	numSessions := 10
	numSavesPerSession := 5

	var wg sync.WaitGroup
	var errorCount int64

	for i := 0; i < numSessions; i++ {
		wg.Add(1)
		go func(sessionID int) {
			defer wg.Done()

			sessionKey := "session-" + string(rune('A'+sessionID))
			sess, err := mgr.GetOrCreate(sessionKey)
			if err != nil {
				t.Errorf("Failed to create session %s: %v", sessionKey, err)
				atomic.AddInt64(&errorCount, 1)
				return
			}

			for j := 0; j < numSavesPerSession; j++ {
				sess.AddMessage(Message{
					Role:      "user",
					Content:   "Message",
					Timestamp: time.Now(),
				})

				if err := mgr.Save(sess); err != nil {
					t.Errorf("Save failed for session %s, iteration %d: %v", sessionKey, j, err)
					atomic.AddInt64(&errorCount, 1)
				}
			}
		}(i)
	}

	wg.Wait()

	if errorCount > 0 {
		t.Errorf("Total save errors: %d", errorCount)
	}

	list, err := mgr.List()
	if err != nil {
		t.Fatalf("Failed to list sessions: %v", err)
	}

	if len(list) != numSessions {
		t.Errorf("Expected %d sessions, got %d", numSessions, len(list))
	}
}

func TestManager_SaveWithHighConcurrency(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "session_high_concurrent_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	mgr, err := NewManager(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	sess, err := mgr.GetOrCreate("high-concurrent-session")
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	numGoroutines := 100
	var wg sync.WaitGroup
	var errorCount int64

	start := make(chan struct{})

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start

			sess.AddMessage(Message{
				Role:      "user",
				Content:   "Concurrent message",
				Timestamp: time.Now(),
			})

			if err := mgr.Save(sess); err != nil {
				atomic.AddInt64(&errorCount, 1)
			}
		}()
	}

	close(start)
	wg.Wait()

	if errorCount > 0 {
		t.Errorf("Total save errors with high concurrency: %d", errorCount)
	}
}

func TestManager_GetSaveMutex(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "session_mutex_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	mgr, err := NewManager(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	mu1 := mgr.getSaveMutex("session-a")
	if mu1 == nil {
		t.Fatal("getSaveMutex returned nil")
	}

	mu2 := mgr.getSaveMutex("session-a")
	if mu1 != mu2 {
		t.Error("getSaveMutex should return the same mutex for the same key")
	}

	mu3 := mgr.getSaveMutex("session-b")
	if mu1 == mu3 {
		t.Error("getSaveMutex should return different mutexes for different keys")
	}
}

func TestManager_RaceCondition(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "session_race_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	mgr, err := NewManager(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	sess, err := mgr.GetOrCreate("race-condition-test")
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	var wg sync.WaitGroup
	done := make(chan bool)

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-done:
					return
				default:
					sess.AddMessage(Message{
						Role:      "user",
						Content:   "Race test message",
						Timestamp: time.Now(),
					})
					_ = mgr.Save(sess)
				}
			}
		}()
	}

	time.Sleep(2 * time.Second)
	close(done)
	wg.Wait()
}
