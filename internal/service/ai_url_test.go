package service

import "testing"

func TestAnthropicMessagesURLNormalizesBase(t *testing.T) {
	cases := map[string]string{
		"":                                "https://api.anthropic.com/v1/messages",
		"https://api.anthropic.com":       "https://api.anthropic.com/v1/messages",
		"https://api.anthropic.com/":      "https://api.anthropic.com/v1/messages",
		"https://api.anthropic.com/v1":    "https://api.anthropic.com/v1/messages",
		"https://api.anthropic.com/v1/":   "https://api.anthropic.com/v1/messages",
		"https://proxy.test/anthropic/v1": "https://proxy.test/anthropic/v1/messages",
		"https://proxy.test/v1/messages":  "https://proxy.test/v1/messages",
	}
	for in, want := range cases {
		if got := anthropicMessagesURL(in); got != want {
			t.Fatalf("anthropicMessagesURL(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestGeminiModelURLNormalizesBase(t *testing.T) {
	base := "https://generativelanguage.googleapis.com/v1beta"
	want := base + "/models/gemini-2.0-flash:generateContent"
	if got := geminiModelURL(base, "gemini-2.0-flash", "generateContent"); got != want {
		t.Fatalf("geminiModelURL = %q, want %q", got, want)
	}
	full := base + "/models/gemini-2.0-flash:generateContent?key=abc"
	if got := geminiModelURL(full, "gemini-2.0-flash", "generateContent"); got != want {
		t.Fatalf("geminiModelURL(full endpoint) = %q, want %q", got, want)
	}
}
