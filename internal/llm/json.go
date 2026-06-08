package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// CallJSON calls the LLM and unmarshals its response into out. It tolerates
// responses wrapped in markdown code fences, surrounded by prose, or containing
// trailing commas (common with smaller local models). If the first response will
// not parse, it retries once with a stricter instruction. Used by every engine
// that asks the model for structured output.
func CallJSON(ctx context.Context, c Caller, system, user string, out any) error {
	raw, err := c.Call(ctx, system, user)
	if err != nil {
		return err
	}
	if err := parseInto(raw, out); err == nil {
		return nil
	}

	// Retry once, echoing the bad output and demanding strict JSON.
	retryUser := user + "\n\nYour previous response could not be parsed as JSON:\n" +
		truncate(raw, 500) + "\n\nReturn ONLY a single valid JSON value. No prose, no code fences, no trailing commas."
	raw2, err := c.Call(ctx, system, retryUser)
	if err != nil {
		return err
	}
	if err := parseInto(raw2, out); err != nil {
		return fmt.Errorf("unmarshal model JSON after retry: %w (raw: %q)", err, truncate(raw2, 300))
	}
	return nil
}

func parseInto(raw string, out any) error {
	cleaned := ExtractJSON(raw)
	if cleaned == "" {
		return fmt.Errorf("no JSON found in model response: %q", truncate(raw, 200))
	}
	cleaned = stripTrailingCommas(cleaned)
	if err := json.Unmarshal([]byte(cleaned), out); err != nil {
		return fmt.Errorf("unmarshal model JSON: %w", err)
	}
	return nil
}

// stripTrailingCommas removes commas that immediately precede a closing brace or
// bracket (ignoring whitespace), a frequent invalid-JSON pattern from LLMs. It
// skips characters inside string literals.
func stripTrailingCommas(s string) string {
	out := make([]byte, 0, len(s))
	inStr := false
	escaped := false
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if escaped {
			out = append(out, ch)
			escaped = false
			continue
		}
		switch {
		case ch == '\\' && inStr:
			out = append(out, ch)
			escaped = true
			continue
		case ch == '"':
			inStr = !inStr
		case ch == ',' && !inStr:
			// Look ahead past whitespace for a closing token.
			j := i + 1
			for j < len(s) && (s[j] == ' ' || s[j] == '\n' || s[j] == '\r' || s[j] == '\t') {
				j++
			}
			if j < len(s) && (s[j] == '}' || s[j] == ']') {
				continue // drop the comma
			}
		}
		out = append(out, ch)
	}
	return string(out)
}

// ExtractJSON pulls a JSON value out of a model response, stripping ```json
// fences and any leading/trailing prose. Returns "" if none is found.
// It also supports auto-repairing truncated JSON (e.g. from output length limits)
// by tracking the nesting stack of open braces/brackets.
func ExtractJSON(s string) string {
	s = strings.TrimSpace(s)

	// Strip markdown code fences if present.
	if strings.HasPrefix(s, "```") {
		if i := strings.IndexByte(s, '\n'); i != -1 {
			s = s[i+1:]
		}
		if j := strings.LastIndex(s, "```"); j != -1 {
			s = s[:j]
		}
		s = strings.TrimSpace(s)
	}

	// Find the outermost { } or [ ] span.
	start := strings.IndexAny(s, "{[")
	if start == -1 {
		return ""
	}

	depth := 0
	inStr := false
	escaped := false
	stack := []byte{} // to keep track of expected closing characters

	for i := start; i < len(s); i++ {
		ch := s[i]
		switch {
		case escaped:
			escaped = false
		case ch == '\\' && inStr:
			escaped = true
		case ch == '"':
			inStr = !inStr
		case inStr:
			// skip
		case ch == '{':
			depth++
			stack = append(stack, '}')
		case ch == '[':
			depth++
			stack = append(stack, ']')
		case ch == '}':
			depth--
			if len(stack) > 0 && stack[len(stack)-1] == '}' {
				stack = stack[:len(stack)-1]
			}
			if depth == 0 {
				return s[start : i+1]
			}
		case ch == ']':
			depth--
			if len(stack) > 0 && stack[len(stack)-1] == ']' {
				stack = stack[:len(stack)-1]
			}
			if depth == 0 {
				return s[start : i+1]
			}
		}
	}

	// If the loop finished and depth > 0, the JSON was truncated.
	// We can auto-repair it by closing quotes and popping brackets/braces.
	if depth > 0 {
		var sb strings.Builder
		temp := s[start:]
		// If it ended on an unfinished escape backslash, drop it so it doesn't escape our closing quote.
		if len(temp) > 0 && temp[len(temp)-1] == '\\' {
			temp = temp[:len(temp)-1]
		}
		sb.WriteString(temp)
		if inStr {
			sb.WriteByte('"')
		}
		for j := len(stack) - 1; j >= 0; j-- {
			sb.WriteByte(stack[j])
		}
		return sb.String()
	}

	return ""
}

// MarshalCompact renders v as compact JSON for embedding in a prompt. On error
// it returns a JSON string describing the failure rather than panicking, so it
// is safe to use inline in prompt construction.
func MarshalCompact(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%q", "marshal error: "+err.Error())
	}
	return string(b)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
