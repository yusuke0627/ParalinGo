package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGenerateLearning(t *testing.T) {
	sentences := []string{"Small steps make a difference."}
	result := LearningResult{Translations: []string{"小さな一歩が違いを生みます。"}, Chunks: []Chunk{{Text: "make a difference", Meaning: "違いを生む", Explanation: "変化をもたらすこと", Sentence: 0}}, SummaryEN: "Small steps matter.", SummaryJA: "小さな一歩が大切です。"}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" || r.Header.Get("Authorization") != "Bearer test-only" {
			t.Errorf("unexpected request")
		}
		var input struct {
			Messages []map[string]string `json:"messages"`
		}
		if json.NewDecoder(r.Body).Decode(&input) != nil || len(input.Messages) != 2 {
			t.Error("missing prompt")
		}
		content, _ := json.Marshal(result)
		json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]string{"content": string(content)}, "finish_reason": "stop"}}})
	}))
	defer server.Close()
	l := &Learning{}
	if err := l.Configure(AISettings{Endpoint: server.URL + "/v1", Model: "mock", APIKey: "test-only"}); err != nil {
		t.Fatal(err)
	}
	got, err := l.Generate(sentences, "TOEIC 600")
	if err != nil {
		t.Fatal(err)
	}
	if got.Translations[0] != result.Translations[0] {
		t.Fatal("translation lost")
	}
	result.Chunks[0].Text = "invented phrase"
	if _, err = l.Generate(sentences, "TOEIC 600"); err == nil {
		t.Fatal("accepted hallucinated chunk")
	}
	result.Chunks = nil
	result.Translations = nil
	if _, err = l.Generate(sentences, "TOEIC 600"); err == nil {
		t.Fatal("accepted missing translation")
	}
}
func TestAIEndpointAndUnconfigured(t *testing.T) {
	l := &Learning{}
	if _, err := l.Generate([]string{"Hello."}, ""); err == nil {
		t.Fatal("missing configuration accepted")
	}
	for _, endpoint := range []string{"http://example.org/v1", "https://user:secret@example.org/v1", "javascript:alert(1)", "https://example.org/v1?key=secret"} {
		if l.Configure(AISettings{Endpoint: endpoint, Model: "test"}) == nil {
			t.Errorf("accepted %q", endpoint)
		}
	}
}
