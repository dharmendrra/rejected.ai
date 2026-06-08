package llm

import (
	"context"
	"testing"
)

func TestExtractJSON(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{"plain", `{"a":1}`, `{"a":1}`},
		{"fenced", "```json\n{\"a\":1}\n```", `{"a":1}`},
		{"prose around", "Here you go:\n{\"a\":1}\nHope that helps", `{"a":1}`},
		{"array", `[1,2,3]`, `[1,2,3]`},
		{"nested", `{"a":{"b":[1,2]},"c":3}`, `{"a":{"b":[1,2]},"c":3}`},
		{"brace in string", `{"a":"}"}`, `{"a":"}"}`},
		{"none", `no json here`, ``},
		{"truncated", `{"name": "DHARMENDRA", "experience": ["Brevo", "Sendin`, `{"name": "DHARMENDRA", "experience": ["Brevo", "Sendin"]}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ExtractJSON(c.in); got != c.want {
				t.Errorf("ExtractJSON(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestStripTrailingCommas(t *testing.T) {
	cases := []struct{ in, want string }{
		{`{"a":1,}`, `{"a":1}`},
		{`[1,2,3,]`, `[1,2,3]`},
		{`{"a":[1,2,],"b":3,}`, `{"a":[1,2],"b":3}`},
		{`{"a":"1,}"}`, `{"a":"1,}"}`}, // comma inside string preserved
		{`{"a":1}`, `{"a":1}`},
	}
	for _, c := range cases {
		if got := stripTrailingCommas(c.in); got != c.want {
			t.Errorf("stripTrailingCommas(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// seqCaller returns successive scripted responses, one per call.
type seqCaller struct {
	resps []string
	i     int
}

func (s *seqCaller) Call(ctx context.Context, system, user string) (string, error) {
	r := s.resps[s.i]
	if s.i < len(s.resps)-1 {
		s.i++
	}
	return r, nil
}
func (s *seqCaller) ModelName() string { return "seq" }

func TestCallJSON_RetryOnBadOutput(t *testing.T) {
	// First response is unparseable; retry returns valid JSON with a trailing comma.
	c := &seqCaller{resps: []string{
		"sorry, here's my answer but not json",
		"```json\n{\"decision\":\"hire\",\"score\":0.8,}\n```",
	}}
	var out struct {
		Decision string  `json:"decision"`
		Score    float64 `json:"score"`
	}
	if err := CallJSON(context.Background(), c, "sys", "user", &out); err != nil {
		t.Fatalf("CallJSON: %v", err)
	}
	if out.Decision != "hire" || out.Score != 0.8 {
		t.Errorf("got %+v, want {hire 0.8}", out)
	}
	if c.i != 1 {
		t.Errorf("expected a retry (i=1), got i=%d", c.i)
	}
}
