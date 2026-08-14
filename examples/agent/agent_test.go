package main

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"google.golang.org/adk/v2/agent"
)

type mockContext struct {
	agent.StrictContextMock
	context.Context
}

func (m *mockContext) Deadline() (deadline time.Time, ok bool) {
	return m.Context.Deadline()
}

func (m *mockContext) Done() <-chan struct{} {
	return m.Context.Done()
}

func (m *mockContext) Err() error {
	return m.Context.Err()
}

func (m *mockContext) Value(key any) any {
	return m.Context.Value(key)
}

func TestGetProcesses(t *testing.T) {
	ctx := &mockContext{
		Context: context.Background(),
	}
	res, err := getProcesses(ctx, struct{}{})
	if err != nil {
		t.Fatalf("getProcesses failed: %v", err)
	}

	output := res.Processes
	fmt.Printf("Processes output:\n%s\n", output)

	// Since TOON format wraps array outputs as [count]{headers}: followed by data,
	// we check for the structure.
	if !strings.HasPrefix(output, "[") {
		t.Errorf("Expected TOON format to start with '[' (array definition), got: %q", output)
	}

	expectedHeaders := "{pid,name,uid,username,memory_bytes,vsize_bytes,cpu_time_seconds}:"
	if !strings.Contains(output, expectedHeaders) {
		t.Errorf("Expected TOON format to contain headers %q, got: %q", expectedHeaders, output)
	}

	// Make sure we have some rows of process data.
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) < 2 {
		t.Errorf("Expected at least 2 lines of output (header + at least one process), got %d lines", len(lines))
	}
}
