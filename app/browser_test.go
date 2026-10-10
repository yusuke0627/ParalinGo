package main

import "testing"

func TestValidateWebURL(t *testing.T) {
	for _, raw := range []string{"https://example.org/article?q=read", "http://localhost:8000/page"} {
		if _, err := validateWebURL(raw); err != nil {
			t.Errorf("valid URL %q: %v", raw, err)
		}
	}
	for _, raw := range []string{"", "javascript:alert(1)", "file:///tmp/article", "https://", "https://user:password@example.org", "example.org"} {
		if _, err := validateWebURL(raw); err == nil {
			t.Errorf("accepted invalid URL %q", raw)
		}
	}
}
