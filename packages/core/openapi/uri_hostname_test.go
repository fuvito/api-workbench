package openapi

import (
	"strings"
	"testing"
)

func TestURIFormat(t *testing.T) {
	for _, good := range []string{
		"https://example.com/a?b=1#c", "http://localhost:8080", "http://[::1]:80/", "mailto:a@example.com",
		"urn:isbn:0451450523", "file:///tmp/a", "tag:example.com,2024:x", "https://example.com/a%20b",
		"https://example.com/café", "data:text/plain;base64,aGk=",
	} {
		if err := validateURI(good); err != nil {
			t.Errorf("%q: %v", good, err)
		}
	}
	for _, bad := range []string{
		"", "not a uri", "/users/1", "users/1", "//example.com/a", "example.com", "https://exa mple.com",
		"https://example.com/a b", "https://example.com/%zz", "https://example.com/100%", "https://x/<y>",
		"http://h:port/", "https://x/\n", ":nothing", "1ttp://x",
	} {
		if validateURI(bad) == nil {
			t.Errorf("%q passed", bad)
		}
	}
}

func TestHostnameFormat(t *testing.T) {
	label := strings.Repeat("a", 63)
	for _, good := range []string{"example.com", "localhost", "a-b.example.co", "xn--caf-dma.com", "example.com.", "192.168.1.1", label + ".com"} {
		if err := validateHostname(good); err != nil {
			t.Errorf("%q: %v", good, err)
		}
	}
	for _, bad := range []string{
		"", ".", "exa mple.com", "-a.com", "a-.com", "a..com", "under_score.com", "http://example.com",
		"example.com:80", "example.com/path", "café.com", label + "a.com", strings.Repeat("abcdefgh.", 32),
	} {
		if validateHostname(bad) == nil {
			t.Errorf("%q passed", bad)
		}
	}
}
