// Package parser loads a canonical plugin source tree into the model IR. It is
// the only layer that understands the on-disk source format.
package parser

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/asingamaneni/omniplug/internal/model"
	"gopkg.in/yaml.v3"
)

// Default source layout (conventional directories).
const (
	manifestFile = "plugin.yaml"
	skillsDir    = "skills"
	commandsDir  = "commands"
	agentsDir    = "agents"
	hooksFile    = "hooks/hooks.yaml"
	mcpFile      = "mcp/servers.yaml"
	guidanceFile = "guidance/AGENTS.md"
	skillFile    = "SKILL.md"
)

// Load parses the plugin rooted at dir into a model.Plugin.
func Load(dir string) (*model.Plugin, error) {
	man, err := loadManifest(filepath.Join(dir, manifestFile))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("no %s found in %s (run 'omniplug init' to scaffold a plugin)", manifestFile, dir)
		}
		return nil, err
	}

	p := &model.Plugin{
		APIVersion:  man.APIVersion,
		Name:        man.Name,
		Version:     man.Version,
		Description: man.Description,
		Author:      model.Author{Name: man.Author.Name, URL: man.Author.URL},
		License:     man.License,
		Homepage:    man.Homepage,
		Repository:  man.Repository,
		Keywords:    man.Keywords,
		Targets:     man.Targets,
	}
	if p.Version == "" {
		p.Version = "0.0.0"
	}

	if p.Skills, err = loadSkills(filepath.Join(dir, skillsDir)); err != nil {
		return nil, err
	}
	if p.Commands, err = loadCommands(filepath.Join(dir, commandsDir)); err != nil {
		return nil, err
	}
	if p.Agents, err = loadAgents(filepath.Join(dir, agentsDir)); err != nil {
		return nil, err
	}
	if p.Hooks, err = loadHooks(filepath.Join(dir, hooksFile)); err != nil {
		return nil, err
	}
	if p.HookFiles, err = loadHookFiles(dir); err != nil {
		return nil, err
	}
	if p.MCPServers, err = loadMCP(filepath.Join(dir, mcpFile)); err != nil {
		return nil, err
	}
	if p.Guidance, err = loadGuidance(filepath.Join(dir, guidanceFile)); err != nil {
		return nil, err
	}

	return p, nil
}

// ---- manifest ----

type rawManifest struct {
	APIVersion  string `yaml:"apiVersion"`
	Name        string `yaml:"name"`
	Version     string `yaml:"version"`
	Description string `yaml:"description"`
	Author      struct {
		Name string `yaml:"name"`
		URL  string `yaml:"url"`
	} `yaml:"author"`
	License    string                            `yaml:"license"`
	Homepage   string                            `yaml:"homepage"`
	Repository string                            `yaml:"repository"`
	Keywords   []string                          `yaml:"keywords"`
	Targets    map[string]map[string]interface{} `yaml:"targets"`
}

func loadManifest(path string) (*rawManifest, error) {
	data, err := readFileCapped(path)
	if err != nil {
		return nil, fmt.Errorf("reading manifest: %w", err)
	}
	var m rawManifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", manifestFile, err)
	}
	return &m, nil
}

// ---- skills ----

type rawSkillFM struct {
	Name            string                            `yaml:"name"`
	Description     string                            `yaml:"description"`
	WhenToUse       string                            `yaml:"whenToUse"`
	ArgumentHint    string                            `yaml:"argumentHint"`
	Arguments       stringList                        `yaml:"arguments"`
	AutoInvoke      *bool                             `yaml:"autoInvoke"`
	UserInvocable   *bool                             `yaml:"userInvocable"`
	AllowedTools    stringList                        `yaml:"allowedTools"`
	DisallowedTools stringList                        `yaml:"disallowedTools"`
	Model           string                            `yaml:"model"`
	Effort          string                            `yaml:"effort"`
	Globs           stringList                        `yaml:"globs"`
	RunInSubagent   bool                              `yaml:"runInSubagent"`
	Targets         map[string]map[string]interface{} `yaml:"targets"`
}

func loadSkills(dir string) ([]model.Skill, error) {
	entries, err := readDirSorted(dir)
	if err != nil {
		return nil, err
	}
	var skills []model.Skill
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		skillDir := filepath.Join(dir, e.Name())
		mdPath := filepath.Join(skillDir, skillFile)
		fmRaw, body, err := splitFrontmatter(mdPath)
		if err != nil {
			return nil, err
		}
		var fm rawSkillFM
		if err := yaml.Unmarshal(fmRaw, &fm); err != nil {
			return nil, fmt.Errorf("parsing frontmatter %s: %w", mdPath, err)
		}
		name := fm.Name
		if name == "" {
			name = e.Name()
		}
		files, err := collectSupportingFiles(skillDir, skillFile)
		if err != nil {
			return nil, err
		}
		skills = append(skills, model.Skill{
			Name:            name,
			Description:     fm.Description,
			WhenToUse:       fm.WhenToUse,
			ArgumentHint:    fm.ArgumentHint,
			Arguments:       fm.Arguments,
			AutoInvoke:      fm.AutoInvoke,
			UserInvocable:   fm.UserInvocable,
			AllowedTools:    fm.AllowedTools,
			DisallowedTools: fm.DisallowedTools,
			Model:           model.ModelTier(fm.Model),
			Effort:          fm.Effort,
			Globs:           fm.Globs,
			RunInSubagent:   fm.RunInSubagent,
			Body:            body,
			Files:           files,
			Targets:         fm.Targets,
		})
	}
	return skills, nil
}

// ---- commands ----

type rawCommandFM struct {
	Name         string                            `yaml:"name"`
	Description  string                            `yaml:"description"`
	ArgumentHint string                            `yaml:"argumentHint"`
	AllowedTools stringList                        `yaml:"allowedTools"`
	Model        string                            `yaml:"model"`
	Targets      map[string]map[string]interface{} `yaml:"targets"`
}

func loadCommands(dir string) ([]model.Command, error) {
	files, err := readMarkdownFiles(dir)
	if err != nil {
		return nil, err
	}
	var cmds []model.Command
	for _, path := range files {
		fmRaw, body, err := splitFrontmatter(path)
		if err != nil {
			return nil, err
		}
		var fm rawCommandFM
		if err := yaml.Unmarshal(fmRaw, &fm); err != nil {
			return nil, fmt.Errorf("parsing frontmatter %s: %w", path, err)
		}
		name := fm.Name
		if name == "" {
			name = baseName(path)
		}
		cmds = append(cmds, model.Command{
			Name:         name,
			Description:  fm.Description,
			ArgumentHint: fm.ArgumentHint,
			AllowedTools: fm.AllowedTools,
			Model:        model.ModelTier(fm.Model),
			Body:         body,
			Targets:      fm.Targets,
		})
	}
	return cmds, nil
}

// ---- agents ----

type rawAgentFM struct {
	Name            string                            `yaml:"name"`
	Description     string                            `yaml:"description"`
	Tools           stringList                        `yaml:"tools"`
	DisallowedTools stringList                        `yaml:"disallowedTools"`
	Model           string                            `yaml:"model"`
	MaxTurns        int                               `yaml:"maxTurns"`
	Color           string                            `yaml:"color"`
	Targets         map[string]map[string]interface{} `yaml:"targets"`
}

func loadAgents(dir string) ([]model.Agent, error) {
	files, err := readMarkdownFiles(dir)
	if err != nil {
		return nil, err
	}
	var agents []model.Agent
	for _, path := range files {
		fmRaw, body, err := splitFrontmatter(path)
		if err != nil {
			return nil, err
		}
		var fm rawAgentFM
		if err := yaml.Unmarshal(fmRaw, &fm); err != nil {
			return nil, fmt.Errorf("parsing frontmatter %s: %w", path, err)
		}
		name := fm.Name
		if name == "" {
			name = baseName(path)
		}
		agents = append(agents, model.Agent{
			Name:            name,
			Description:     fm.Description,
			Tools:           fm.Tools,
			DisallowedTools: fm.DisallowedTools,
			Model:           model.ModelTier(fm.Model),
			MaxTurns:        fm.MaxTurns,
			Color:           fm.Color,
			Body:            body,
			Targets:         fm.Targets,
		})
	}
	return agents, nil
}

// ---- hooks ----

type rawHooks struct {
	Hooks []struct {
		Event   string `yaml:"event"`
		Matcher string `yaml:"matcher"`
		Type    string `yaml:"type"`
		Command string `yaml:"command"`
	} `yaml:"hooks"`
}

func loadHooks(path string) ([]model.Hook, error) {
	data, err := readFileCapped(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var rh rawHooks
	if err := yaml.Unmarshal(data, &rh); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", hooksFile, err)
	}
	var hooks []model.Hook
	for _, h := range rh.Hooks {
		typ := h.Type
		if typ == "" {
			typ = "command"
		}
		hooks = append(hooks, model.Hook{Event: h.Event, Matcher: h.Matcher, Type: typ, Command: h.Command})
	}
	return hooks, nil
}

// loadHookFiles collects bundled hook scripts under hooks/ (everything except
// hooks.yaml), with paths relative to the plugin root so they can be re-emitted
// at the same location and referenced by `./hooks/...` commands.
func loadHookFiles(pluginRoot string) ([]model.File, error) {
	hooksRoot := filepath.Join(pluginRoot, "hooks")
	var files []model.File
	err := filepath.Walk(hooksRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(pluginRoot, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == hooksFile { // skip hooks/hooks.yaml
			return nil
		}
		content, err := readFileCapped(path)
		if err != nil {
			return err
		}
		files = append(files, model.File{RelPath: rel, Content: content, Mode: info.Mode().Perm()})
		return nil
	})
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	sort.Slice(files, func(i, j int) bool { return files[i].RelPath < files[j].RelPath })
	return files, nil
}

// ---- mcp ----

type rawMCP struct {
	Servers []struct {
		Name      string            `yaml:"name"`
		Transport string            `yaml:"transport"`
		Command   string            `yaml:"command"`
		Args      []string          `yaml:"args"`
		Env       map[string]string `yaml:"env"`
		URL       string            `yaml:"url"`
	} `yaml:"servers"`
}

func loadMCP(path string) ([]model.MCPServer, error) {
	data, err := readFileCapped(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var rm rawMCP
	if err := yaml.Unmarshal(data, &rm); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", mcpFile, err)
	}
	var servers []model.MCPServer
	for _, s := range rm.Servers {
		transport := s.Transport
		if transport == "" {
			transport = "stdio"
		}
		servers = append(servers, model.MCPServer{
			Name: s.Name, Transport: transport, Command: s.Command,
			Args: s.Args, Env: s.Env, URL: s.URL,
		})
	}
	return servers, nil
}

// ---- guidance ----

func loadGuidance(path string) (*model.Guidance, error) {
	data, err := readFileCapped(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &model.Guidance{Body: string(data)}, nil
}

// ---- helpers ----

// splitFrontmatter reads a markdown file and separates leading YAML frontmatter
// (delimited by lines that are exactly `---`) from the body. Frontmatter is
// optional. The closing delimiter must be a full line so that a body containing
// a Markdown horizontal rule or a line beginning with `---` is not mis-split.
func splitFrontmatter(path string) (fm []byte, body string, err error) {
	data, err := readFileCapped(path)
	if err != nil {
		return nil, "", fmt.Errorf("reading %s: %w", path, err)
	}
	// Normalize CRLF to LF for deterministic parsing.
	data = bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
	if !bytes.HasPrefix(data, []byte("---\n")) {
		return nil, strings.TrimLeft(string(data), "\n"), nil
	}
	rest := data[len("---\n"):]
	// Scan line by line for a closing fence consisting solely of "---".
	for pos := 0; pos <= len(rest); {
		nl := bytes.IndexByte(rest[pos:], '\n')
		lineEnd := len(rest)
		if nl >= 0 {
			lineEnd = pos + nl
		}
		line := strings.TrimRight(string(rest[pos:lineEnd]), "\r")
		if line == "---" {
			fm = rest[:pos]
			if nl < 0 {
				return fm, "", nil
			}
			return fm, strings.TrimLeft(string(rest[lineEnd+1:]), "\n"), nil
		}
		if nl < 0 {
			break
		}
		pos = lineEnd + 1
	}
	return nil, "", fmt.Errorf("unterminated frontmatter in %s", path)
}

// maxFileBytes caps the size of any single file read from a (possibly untrusted)
// plugin source, to bound memory use against a malicious oversized file.
const maxFileBytes = 10 << 20 // 10 MiB

// readFileCapped reads a regular file, refusing symlinks (which could point
// outside the source tree, e.g. ~/.ssh/id_rsa) and files larger than
// maxFileBytes. It reads through a single fd with an io.LimitReader, so there is
// no stat-then-read size TOCTOU.
func readFileCapped(path string) ([]byte, error) {
	fi, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("%s is a symlink; refusing to read", path)
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxFileBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxFileBytes {
		return nil, fmt.Errorf("%s exceeds the %d-byte limit", path, maxFileBytes)
	}
	return data, nil
}

func readDirSorted(dir string) ([]os.DirEntry, error) {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	return entries, nil
}

func readMarkdownFiles(dir string) ([]string, error) {
	entries, err := readDirSorted(dir)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		out = append(out, filepath.Join(dir, e.Name()))
	}
	return out, nil
}

// collectSupportingFiles returns every file under root except the named entry,
// with slash-separated relative paths, sorted for determinism.
func collectSupportingFiles(root, except string) ([]model.File, error) {
	var files []model.File
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		// Skip symlinks: reading through them could exfiltrate files outside the
		// source tree (e.g. a link to ~/.ssh/id_rsa) into the generated bundle.
		if info.Mode()&os.ModeSymlink != 0 {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == except {
			return nil
		}
		content, err := readFileCapped(path)
		if err != nil {
			return err
		}
		files = append(files, model.File{RelPath: rel, Content: content, Mode: info.Mode().Perm()})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(files, func(i, j int) bool { return files[i].RelPath < files[j].RelPath })
	return files, nil
}

func baseName(path string) string {
	b := filepath.Base(path)
	return strings.TrimSuffix(b, filepath.Ext(b))
}

// stringList accepts either a YAML sequence or a scalar string (split on
// whitespace or commas), so authors can write `allowedTools: Read Grep` or a list.
type stringList []string

func (s *stringList) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.SequenceNode:
		var items []string
		if err := value.Decode(&items); err != nil {
			return err
		}
		*s = items
	case yaml.ScalarNode:
		raw := strings.TrimSpace(value.Value)
		if raw == "" {
			*s = nil
			return nil
		}
		*s = splitTools(raw)
	default:
		return fmt.Errorf("expected list or string, got %v", value.Kind)
	}
	return nil
}

// splitTools splits a scalar tool list on whitespace and commas, but treats
// parentheses as grouping so patterns like `Bash(git push *)` stay intact.
func splitTools(raw string) []string {
	var items []string
	var cur strings.Builder
	depth := 0
	flush := func() {
		if cur.Len() > 0 {
			items = append(items, cur.String())
			cur.Reset()
		}
	}
	for _, r := range raw {
		switch {
		case r == '(':
			depth++
			cur.WriteRune(r)
		case r == ')':
			if depth > 0 {
				depth--
			}
			cur.WriteRune(r)
		case (r == ' ' || r == '\t' || r == ',') && depth == 0:
			flush()
		default:
			cur.WriteRune(r)
		}
	}
	flush()
	return items
}
