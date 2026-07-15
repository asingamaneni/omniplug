// Package yamlfm builds YAML frontmatter deterministically. Adapters use it to
// emit `---`-delimited frontmatter for skills, commands, agents, and rules.
//
// Scalars are emitted as JSON-quoted strings, which are also valid YAML
// double-quoted scalars — safe for values containing colons, quotes, or other
// YAML metacharacters.
package yamlfm

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// Builder accumulates frontmatter fields in insertion order.
type Builder struct {
	lines []string
}

// Scalar appends `key: "value"` if value is non-empty.
func (b *Builder) Scalar(key, value string) {
	if value == "" {
		return
	}
	b.lines = append(b.lines, key+": "+Quote(value))
}

// Raw appends `key: value` verbatim if value is non-empty (for safe enums/numbers).
func (b *Builder) Raw(key, value string) {
	if value == "" {
		return
	}
	b.lines = append(b.lines, key+": "+value)
}

// RawField appends `key: value`, or `key:` when value is empty. Use when the
// target requires the key to be present even with an empty value.
func (b *Builder) RawField(key, value string) {
	if value == "" {
		b.lines = append(b.lines, key+":")
		return
	}
	b.lines = append(b.lines, key+": "+value)
}

// Bool appends `key: true|false` unconditionally.
func (b *Builder) Bool(key string, v bool) {
	b.lines = append(b.lines, fmt.Sprintf("%s: %t", key, v))
}

// List appends `key: ["a", "b"]` (flow sequence) if the slice is non-empty.
func (b *Builder) List(key string, items []string) {
	if len(items) == 0 {
		return
	}
	quoted := make([]string, len(items))
	for i, it := range items {
		quoted[i] = Quote(it)
	}
	b.lines = append(b.lines, key+": ["+strings.Join(quoted, ", ")+"]")
}

// Targets appends raw per-target override fields (escape hatch), sorted by key.
func (b *Builder) Targets(raw map[string]interface{}) {
	if len(raw) == 0 {
		return
	}
	keys := make([]string, 0, len(raw))
	for k := range raw {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v, err := json.Marshal(raw[k])
		if err != nil {
			continue
		}
		// Quote the (untrusted) key so it cannot inject extra frontmatter lines
		// via embedded newlines or YAML metacharacters. The value is JSON-encoded,
		// which is valid YAML for scalars, sequences, and maps alike.
		b.lines = append(b.lines, Quote(k)+": "+string(v))
	}
}

// Render wraps the collected fields in --- delimiters and appends the body.
func (b *Builder) Render(body string) []byte {
	var sb strings.Builder
	sb.WriteString("---\n")
	for _, l := range b.lines {
		sb.WriteString(l)
		sb.WriteByte('\n')
	}
	sb.WriteString("---\n")
	if body != "" {
		sb.WriteString("\n")
		sb.WriteString(strings.TrimRight(body, "\n"))
		sb.WriteString("\n")
	}
	return []byte(sb.String())
}

// Quote returns a JSON-quoted string, valid as a YAML double-quoted scalar.
func Quote(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
