package filterusername

import (
	"errors"
	"strings"
	"testing"

	"github.com/openSUSE/piiplugin/filter"
)

// TestNewUsernameFilter_Getent checks that the pure filter redacts and does not
// redact the expected names when the user database is read through getent.
func TestNewUsernameFilter_Getent(t *testing.T) {
	filter.UseMock = true
	defer func() {
		filter.UseMock = false
	}()

	getpasswdMock := func() ([]string, error) {
		return []string{
			"jdoe:x:1001:1001:John Doe,Room 101,,:/home/jdoe:/bin/bash",
			"alice:x:1002:1002:Alice InWonderland,,:/home/alice:/bin/bash",
			"system:x:500:500:System User,,:/home/system:/bin/bash", // should be ignored (UID < 1000)
		}, nil
	}

	f, err := NewUsernameFilter(
		WithGetpasswdFunc(getpasswdMock),
		WithUsernameSource(SourceGetent),
	)
	if err != nil {
		t.Fatalf("Failed to create UsernameFilter: %v", err)
	}

	inputText := "Hello John, your username is jdoe. Is Alice here? But do not replace Johnathan or DoeMaster. And System must not be replaced."
	redacted := f.Redact(inputText, inputText)

	// With the mock generator, replacements are the reversed original, so the
	// redacted text must contain the reversed forms of the known names.
	for _, name := range []string{"nhoJ", "eodj", "ecilA"} {
		if !strings.Contains(redacted, name) {
			t.Errorf("Expected %q to be present after redaction, got: %q", name, redacted)
		}
	}

	// System users (UID < 1000) and longer words that merely share a prefix must
	// be left alone.
	for _, word := range []string{"System", "Johnathan", "DoeMaster"} {
		if !strings.Contains(redacted, word) {
			t.Errorf("Expected %q to be kept, got: %q", word, redacted)
		}
	}

	// Round-trip must be lossless.
	unredacted := f.Unredact(redacted)
	if unredacted != inputText {
		t.Errorf("Round-trip failed.\nExpected: %q\nGot:      %q", inputText, unredacted)
	}
}

// TestUniqueNames checks the internal uniqueness helper.
func TestUniqueNames(t *testing.T) {
	input := []string{"John", "john", "DOE", "Doe", "alice"}
	expected := []string{"John", "DOE", "alice"}

	output := uniqueNames(input)
	if len(output) != len(expected) {
		t.Fatalf("Expected unique names length %d, got %d", len(expected), len(output))
	}
	for i, v := range output {
		if strings.ToLower(v) != strings.ToLower(expected[i]) {
			t.Errorf("At index %d, expected %q, got %q", i, expected[i], v)
		}
	}
}

// TestSourceOptions ensures the source option is validated and error path works
// when forcing cgo in a non-CGO build (compile-time dependent). Under CGO builds
// this just exercises the auto path.
func TestSourceAuto(t *testing.T) {
	// SourceAuto should work regardless of CGO availability.
	if _, err := NewUsernameFilter(WithUsernameSource(SourceAuto)); err != nil {
		t.Fatalf("SourceAuto should be available in any build, got: %v", err)
	}
}

// TestSourceCgoForce in a CGO-enabled build must succeed; in a non-CGO build it
// must return a clear error instead of silently doing the wrong thing.
func TestSourceCgoForce(t *testing.T) {
	_, err := NewUsernameFilter(WithUsernameSource(SourceCgo))
	switch cgoSource {
	case nil:
		if err == nil {
			t.Fatal("Expected an error when forcing cgo in a non-CGO build, got nil")
		}
		if !strings.Contains(err.Error(), "CGO") {
			t.Errorf("Expected an error mentioning CGO, got: %v", err)
		}
	default:
		if err != nil {
			t.Fatalf("Expected cgo source to work in a CGO build, got: %v", err)
		}
	}
}

// TestSourceGetentForce is a regression check for the getent path: it must
// produce the same shape as the CGO path (colon-separated lines) and the
// filter must redact the same user names.
func TestSourceGetentForce(t *testing.T) {
	mockLines := []string{
		"jdoe:x:1001:1001:John Doe,Room 101,,:/home/jdoe:/bin/bash",
	}
	f, err := NewUsernameFilter(
		WithUsernameSource(SourceGetent),
		WithGetpasswdFunc(func() ([]string, error) { return mockLines, nil }),
	)
	if err != nil {
		t.Fatalf("Failed to create filter: %v", err)
	}
	redacted := f.Redact("hi jdoe", "hi jdoe")
	if strings.Contains(redacted, "jdoe") {
		t.Errorf("Expected 'jdoe' to be redacted, got: %q", redacted)
	}
}

// TestGetentErrorPropagation ensures an error from the getent command (i.e. a
// real exec failure) is surfaced from the constructor.
func TestGetentErrorPropagation(t *testing.T) {
	// Force the filter through the real FetchGetentPasswd with a fake function
	// returning an error, to verify the error path.
	_, err := NewUsernameFilter(
		WithUsernameSource(SourceGetent),
		WithGetpasswdFunc(func() ([]string, error) { return nil, errors.New("getent: not found") }),
	)
	if err == nil {
		t.Fatal("Expected an error to propagate from the getpasswd function")
	}
}
