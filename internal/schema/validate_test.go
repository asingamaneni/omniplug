package schema

import (
	"testing"

	"github.com/asingamaneni/omniplug/internal/adapter"
	"github.com/asingamaneni/omniplug/internal/model"
)

func TestValidCleanPlugin(t *testing.T) {
	p := &model.Plugin{
		Name: "ok", APIVersion: model.APIVersion,
		Skills: []model.Skill{{Name: "s", Description: "d", Model: model.TierFast}},
	}
	if ds := Validate(p); adapter.HasErrors(ds) {
		t.Errorf("expected no errors, got %+v", ds)
	}
}

func TestMissingDescriptionIsError(t *testing.T) {
	p := &model.Plugin{Name: "ok", Skills: []model.Skill{{Name: "s"}}}
	if !adapter.HasErrors(Validate(p)) {
		t.Error("expected error for missing skill description")
	}
}

func TestInvalidModelTierIsError(t *testing.T) {
	p := &model.Plugin{Name: "ok", Skills: []model.Skill{{Name: "s", Description: "d", Model: "turbo"}}}
	if !adapter.HasErrors(Validate(p)) {
		t.Error("expected error for invalid model tier")
	}
}

func TestApiVersionMismatchIsWarning(t *testing.T) {
	p := &model.Plugin{Name: "ok", APIVersion: "omniplug/v9"}
	ds := Validate(p)
	if adapter.HasErrors(ds) {
		t.Errorf("apiVersion mismatch should warn, not error: %+v", ds)
	}
	if len(ds) == 0 {
		t.Error("expected a warning for apiVersion mismatch")
	}
}

func TestMissingPluginNameIsError(t *testing.T) {
	if !adapter.HasErrors(Validate(&model.Plugin{})) {
		t.Error("expected error for missing plugin name")
	}
}

func TestRejectsTraversalName(t *testing.T) {
	for _, bad := range []string{"../evil", "a/b", "..", "foo/../bar"} {
		p := &model.Plugin{Name: "ok", Skills: []model.Skill{{Name: bad, Description: "d"}}}
		if !adapter.HasErrors(Validate(p)) {
			t.Errorf("expected error for unsafe name %q", bad)
		}
	}
}

func TestRejectsDuplicateNames(t *testing.T) {
	p := &model.Plugin{Name: "ok", Skills: []model.Skill{
		{Name: "dup", Description: "a"}, {Name: "dup", Description: "b"},
	}}
	if !adapter.HasErrors(Validate(p)) {
		t.Error("expected error for duplicate skill name")
	}
}

func TestRejectsDuplicateMCPNames(t *testing.T) {
	p := &model.Plugin{Name: "ok", MCPServers: []model.MCPServer{
		{Name: "gh", Transport: "stdio"}, {Name: "gh", Transport: "http"},
	}}
	if !adapter.HasErrors(Validate(p)) {
		t.Error("expected error for duplicate MCP server name")
	}
}

func TestRejectsInvalidHookTypeAndTransport(t *testing.T) {
	p := &model.Plugin{Name: "ok",
		Hooks:      []model.Hook{{Event: "PostToolUse", Type: "telepathy"}},
		MCPServers: []model.MCPServer{{Name: "s", Transport: "grpc"}},
	}
	if !adapter.HasErrors(Validate(p)) {
		t.Error("expected errors for invalid hook type and transport")
	}
}
