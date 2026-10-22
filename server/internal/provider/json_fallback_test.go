package provider

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestJSONSchemaFallbackUsesJSONObjectAndPrompt(t *testing.T) {
	req := Request{System: "系统", Prompt: "用户", Format: FormatJSONSchema, Schema: json.RawMessage(`{"type":"object","required":["ok"]}`), Strict: true}
	body, berr := buildBody(req, "m", true, "")
	if berr != nil {
		t.Fatal(berr)
	}
	var parsed struct {
		ResponseFormat struct {
			Type string `json:"type"`
		} `json:"response_format"`
		Messages []struct {
			Content string `json:"content"`
		} `json:"messages"`
	}
	if json.Unmarshal(body, &parsed) != nil || parsed.ResponseFormat.Type != "json_object" || len(parsed.Messages) == 0 || !strings.Contains(parsed.Messages[len(parsed.Messages)-1].Content, `"required"`) {
		t.Fatalf("fallback body=%s", body)
	}
	body, berr = buildBody(req, "m", false, "")
	if berr != nil {
		t.Fatal(berr)
	}
	if !strings.Contains(string(body), `"json_schema"`) {
		t.Fatal("默认未使用 json_schema")
	}
}
