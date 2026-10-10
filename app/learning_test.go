package main

import (
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestPlanStream(t *testing.T) {
	raw := `data: {"type":"response.output_text.delta","delta":"hello"}

data: {"type":"response.completed","response":{"status":"completed"}}

`
	got, err := readPlanStream(strings.NewReader(raw))
	if err != nil || got != "hello" {
		t.Fatalf("%q %v", got, err)
	}
	for _, raw := range []string{`data: {"type":"response.output_text.delta","delta":"partial"}

`, `data: {"type":"response.failed"}

`, `data: {"type":"response.incomplete"}

`} {
		if _, err := readPlanStream(strings.NewReader(raw)); err == nil {
			t.Fatal("accepted incomplete output")
		}
	}
}
func TestLoginCallback(t *testing.T) {
	a := &loginAttempt{state: "expected", expires: time.Now().Add(time.Minute)}
	q := url.Values{"state": {"expected"}, "code": {"code"}, "client_id": {"oaiapp_test"}}
	if _, err := validateCallback(a, q); err != nil {
		t.Fatal(err)
	}
	q.Set("state", "wrong")
	if _, err := validateCallback(a, q); err == nil {
		t.Fatal("accepted invalid state")
	}
	q.Set("state", "expected")
	a.client = "oaiapp_other"
	if _, err := validateCallback(a, q); err == nil {
		t.Fatal("accepted changed client")
	}
}
func TestLearningValidation(t *testing.T) {
	input := []string{"Small steps make a difference."}
	result := LearningResult{Translations: []string{"小さな一歩が違いを生みます。"}, SummaryEN: "Small steps matter.", SummaryJA: "小さな一歩が大切です。", Chunks: []Chunk{{Text: "invented", Meaning: "意味", Explanation: "解説", Sentence: 0}}}
	if validateLearning(result, input) == nil {
		t.Fatal("accepted invented phrase")
	}
	result.Chunks = nil
	if err := validateLearning(result, input); err != nil {
		t.Fatal(err)
	}
	result.Translations = nil
	if validateLearning(result, input) == nil {
		t.Fatal("accepted wrong count")
	}
}
func TestPlanScope(t *testing.T) {
	if hasPlanScope("openid profile") || !hasPlanScope("openid chatgpt.tokens.use.direct") {
		t.Fatal("invalid scope check")
	}
}
