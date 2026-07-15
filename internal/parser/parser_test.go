package parser

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/asingamaneni/omniplug/internal/model"
)

const examplePath = "../../examples/hello-plugin"

func loadExample(t *testing.T) *model.Plugin {
	t.Helper()
	p, err := Load(filepath.FromSlash(examplePath))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return p
}

func TestLoadManifest(t *testing.T) {
	p := loadExample(t)
	if p.Name != "hello-plugin" {
		t.Errorf("name = %q, want hello-plugin", p.Name)
	}
	if p.Version != "0.1.0" {
		t.Errorf("version = %q, want 0.1.0", p.Version)
	}
	if p.Author.Name != "Ashok" {
		t.Errorf("author = %q, want Ashok", p.Author.Name)
	}
}

func TestLoadComponents(t *testing.T) {
	p := loadExample(t)
	if len(p.Skills) != 1 {
		t.Fatalf("skills = %d, want 1", len(p.Skills))
	}
	if len(p.Commands) != 1 {
		t.Fatalf("commands = %d, want 1", len(p.Commands))
	}
	if len(p.Agents) != 1 {
		t.Fatalf("agents = %d, want 1", len(p.Agents))
	}
	if len(p.Hooks) != 1 {
		t.Fatalf("hooks = %d, want 1", len(p.Hooks))
	}
	if len(p.MCPServers) != 2 {
		t.Fatalf("mcp servers = %d, want 2", len(p.MCPServers))
	}
	if p.Guidance == nil || p.Guidance.Body == "" {
		t.Error("guidance not loaded")
	}
	if len(p.HookFiles) != 1 || p.HookFiles[0].RelPath != "hooks/scripts/format.sh" {
		t.Errorf("hook files = %v, want [hooks/scripts/format.sh]", p.HookFiles)
	}
}

func TestSkillFields(t *testing.T) {
	p := loadExample(t)
	s := p.Skills[0]
	if s.Name != "summarize-changes" {
		t.Errorf("skill name = %q", s.Name)
	}
	if s.Model != model.TierBalanced {
		t.Errorf("skill model = %q, want balanced", s.Model)
	}
	if len(s.AllowedTools) != 2 {
		t.Errorf("allowedTools = %v, want 2", s.AllowedTools)
	}
	// Supporting file should be collected, not SKILL.md.
	if len(s.Files) != 1 || s.Files[0].RelPath != "scripts/diff.sh" {
		t.Errorf("supporting files = %v, want [scripts/diff.sh]", s.Files)
	}
}

func TestParenAwareToolSplit(t *testing.T) {
	p := loadExample(t)
	c := p.Commands[0]
	if len(c.AllowedTools) != 1 || c.AllowedTools[0] != "Bash(git push *)" {
		t.Errorf("command allowedTools = %v, want [Bash(git push *)]", c.AllowedTools)
	}
}

func TestMCPTransports(t *testing.T) {
	p := loadExample(t)
	byName := map[string]model.MCPServer{}
	for _, s := range p.MCPServers {
		byName[s.Name] = s
	}
	if byName["github"].Transport != "stdio" {
		t.Errorf("github transport = %q, want stdio", byName["github"].Transport)
	}
	if byName["docs"].Transport != "http" || byName["docs"].URL == "" {
		t.Errorf("docs server not parsed as http: %+v", byName["docs"])
	}
}

func TestFrontmatterBodyWithHorizontalRule(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "SKILL.md")
	content := "---\nname: x\ndescription: y\n---\n\nIntro paragraph.\n\n---\n\nSection after a horizontal rule.\n"
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	fm, body, err := splitFrontmatter(p)
	if err != nil {
		t.Fatalf("splitFrontmatter: %v", err)
	}
	if !contains(string(fm), "name: x") || contains(string(fm), "Intro") {
		t.Errorf("frontmatter mis-split: %q", fm)
	}
	if !contains(body, "Section after a horizontal rule") || !contains(body, "---") {
		t.Errorf("body lost the horizontal rule or trailing section: %q", body)
	}
}

func TestReadFileCappedRejectsSymlink(t *testing.T) {
	dir := t.TempDir()
	secret := filepath.Join(dir, "secret")
	if err := os.WriteFile(secret, []byte("top secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link.md")
	if err := os.Symlink(secret, link); err != nil {
		t.Skipf("symlinks not supported: %v", err)
	}
	if _, err := readFileCapped(link); err == nil {
		t.Error("readFileCapped should refuse to read through a symlink")
	}
}

func TestUnterminatedFrontmatterErrors(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "SKILL.md")
	if err := os.WriteFile(p, []byte("---\nname: x\nno closing fence\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := splitFrontmatter(p); err == nil {
		t.Error("expected error for unterminated frontmatter")
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestSplitToolsUnit(t *testing.T) {
	got := splitTools("Bash(git push *) Read, Grep")
	want := []string{"Bash(git push *)", "Read", "Grep"}
	if len(got) != len(want) {
		t.Fatalf("splitTools = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("item %d = %q, want %q", i, got[i], want[i])
		}
	}
}
