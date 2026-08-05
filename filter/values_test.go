package filter

import (
	"strings"
	"testing"
)

// TestMapToolValues_Nested verifies that every string of a tool payload is
// rewritten, no matter how deeply it is nested, and that non-string values are
// left untouched.
func TestMapToolValues_Nested(t *testing.T) {
	payload := map[string]any{
		"processes": []any{
			map[string]any{
				"pid":      42,
				"username": "john",
				"tags":     []string{"john", "root"},
			},
		},
		"host":  "john-laptop",
		"count": 1,
	}

	MapToolValues(payload, strings.ToUpper)

	procs, ok := payload["processes"].([]any)
	if !ok || len(procs) != 1 {
		t.Fatalf("Expected the process list to be preserved, got: %v", payload["processes"])
	}
	proc := procs[0].(map[string]any)
	if proc["username"] != "JOHN" {
		t.Errorf("Expected nested map value to be mapped, got: %v", proc["username"])
	}
	if tags := proc["tags"].([]string); tags[0] != "JOHN" || tags[1] != "ROOT" {
		t.Errorf("Expected string slice to be mapped, got: %v", tags)
	}
	if payload["host"] != "JOHN-LAPTOP" {
		t.Errorf("Expected top level value to be mapped, got: %v", payload["host"])
	}
	if proc["pid"] != 42 || payload["count"] != 1 {
		t.Errorf("Non string values must be kept as they are, got: %v, %v", proc["pid"], payload["count"])
	}
}

// TestToolValuesText checks that all strings of a payload end up in the
// collected text and that the payload itself is not changed by collecting it.
func TestToolValuesText(t *testing.T) {
	payload := map[string]any{
		"user": "john",
		"hosts": []any{
			"first.example.com",
			map[string]any{"name": "second.example.com"},
		},
	}

	text := ToolValuesText(payload)

	for _, want := range []string{"john", "first.example.com", "second.example.com"} {
		if !strings.Contains(text, want) {
			t.Errorf("Expected %q in the collected text, got: %q", want, text)
		}
	}
	if payload["user"] != "john" {
		t.Errorf("Collecting must not change the payload, got: %v", payload["user"])
	}
}
