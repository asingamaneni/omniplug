package yamlfm

import (
	"strings"
	"testing"
)

func TestTargetsKeyCannotInject(t *testing.T) {
	b := &Builder{}
	// A malicious targets key containing a newline must not break out into an
	// extra frontmatter line.
	b.Targets(map[string]interface{}{"a\ninjected: true": 1})
	out := string(b.Render(""))
	if strings.Contains(out, "\ninjected: true") {
		t.Errorf("key injection not prevented:\n%s", out)
	}
	// The newline should be escaped inside a quoted key.
	if !strings.Contains(out, `\ninjected: true`) {
		t.Errorf("expected escaped key, got:\n%s", out)
	}
}

func TestScalarQuotingHandlesColons(t *testing.T) {
	b := &Builder{}
	b.Scalar("description", "Use when asked: what changed?")
	out := string(b.Render(""))
	if !strings.Contains(out, `description: "Use when asked: what changed?"`) {
		t.Errorf("colon value not quoted safely:\n%s", out)
	}
}
