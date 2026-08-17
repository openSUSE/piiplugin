package filterhost

import (
	"strings"
	"testing"
)

func newReplMap() *map[string]string {
	m := make(map[string]string)
	return &m
}

func TestNewHostFilter_Defaults(t *testing.T) {
	f, err := NewHostFilter()
	if err != nil {
		t.Fatalf("Failed to create HostFilter: %v", err)
	}
	if f == nil {
		t.Fatal("Filter should not be nil")
	}
}

func TestHostFilter_RedactAndUnredact(t *testing.T) {
	replacements := newReplMap()
	lookupMock := func(domain string) ([]string, error) {
		return []string{
			"gateway.myoffice.internal.",
			"mailserver.myoffice.internal.",
			"workstation42.myoffice.internal.",
		}, nil
	}

	f, err := NewHostFilter(
		WithReplacement(replacements),
		WithDomain("myoffice.internal"),
		WithDNSServer("192.168.1.1"),
		WithLookupFunc(lookupMock),
	)
	if err != nil {
		t.Fatalf("Failed to create HostFilter: %v", err)
	}

	inputText := "Connect to mailserver.myoffice.internal or gateway. We also have workstation42 and our internal domain is myoffice.internal."
	redacted := f.Redact(inputText, inputText)

	// Each of the four host names must be replaced with a different value.
	for _, name := range []string{
		"mailserver", "gateway", "workstation42", "myoffice",
	} {
		if strings.Contains(redacted, name) {
			t.Errorf("Expected %q to be redacted, got: %q", name, redacted)
		}
	}

	// Round-trip must be lossless.
	unredacted := f.Unredact(redacted)
	if unredacted != inputText {
		t.Errorf("Unredacting failed.\nExpected: %q\nGot:      %q", inputText, unredacted)
	}
}

func TestCleanHostAndDomainNames(t *testing.T) {
	input := []string{
		"myhost.mycompany.local",
		"192.168.1.1", // IP address - should be ignored
		"a",           // too short - should be ignored
	}
	expected := []string{
		"myhost.mycompany.local",
		"myhost",
		"mycompany.local",
	}

	output := cleanHostAndDomainNames(input)
	if len(output) != len(expected) {
		t.Fatalf("Expected %d clean names, got %d: %v", len(expected), len(output), output)
	}
	for i, v := range output {
		if v != expected[i] {
			t.Errorf("At index %d, expected %q, got %q", i, expected[i], v)
		}
	}
}
