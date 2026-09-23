package dockerx

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	if os.Getenv("WPGCTL_FAKE_DOCKER_LOGS") == "1" {
		fmt.Fprintln(os.Stdout, "hello-log")
		_ = os.Stdout.Sync()
		time.Sleep(30 * time.Second)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestLogsFollowStreamsAndCancel(t *testing.T) {
	t.Setenv("WPGCTL_FAKE_DOCKER_LOGS", "1")
	bin, err := os.Executable()
	if err != nil {
		bin = os.Args[0]
	}
	r := New()
	r.Bin = bin
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var lines []string
	var mu sync.Mutex
	done := make(chan error, 1)
	go func() {
		done <- r.LogsFollow(ctx, "any", 20, func(line string) {
			mu.Lock()
			lines = append(lines, line)
			n := len(lines)
			mu.Unlock()
			if n >= 1 {
				cancel()
			}
		})
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(8 * time.Second):
		cancel()
		t.Fatal("timeout waiting for follow cancel")
	}
	mu.Lock()
	defer mu.Unlock()
	if len(lines) == 0 || lines[0] != "hello-log" {
		t.Fatalf("lines %v", lines)
	}
}
