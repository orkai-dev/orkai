package pages

import (
	"encoding/json"
	"testing"
)

func TestResolveTags_Static(t *testing.T) {
	tags := []Tag{{Key: "ManagedBy", Value: "orka'i"}}
	got := ResolveTags(tags, TagContext{})
	if got["ManagedBy"] != "orka'i" {
		t.Fatalf("got %q, want orka'i", got["ManagedBy"])
	}
}

func TestResolveTags_Dynamic(t *testing.T) {
	tags := []Tag{
		{Key: "Environment", Value: "{{env}}"},
		{Key: "Owner", Value: "{{team}}"},
		{Key: "Project", Value: "{{project}}"},
		{Key: "Page", Value: "{{page}}"},
	}
	tc := TagContext{Env: "prod", Team: "platform", Project: "my-app", Page: "landing"}
	got := ResolveTags(tags, tc)
	if got["Environment"] != "prod" {
		t.Fatalf("env: got %q", got["Environment"])
	}
	if got["Owner"] != "platform" {
		t.Fatalf("team: got %q", got["Owner"])
	}
	if got["Project"] != "my-app" {
		t.Fatalf("project: got %q", got["Project"])
	}
	if got["Page"] != "landing" {
		t.Fatalf("page: got %q", got["Page"])
	}
}

func TestResolveTags_MixedStaticAndDynamic(t *testing.T) {
	tags := []Tag{{Key: "Label", Value: "env-{{env}}-team"}}
	got := ResolveTags(tags, TagContext{Env: "qa"})
	if got["Label"] != "env-qa-team" {
		t.Fatalf("got %q", got["Label"])
	}
}

func TestResolveTags_SkipUnresolved(t *testing.T) {
	tags := []Tag{
		{Key: "Environment", Value: "{{env}}"},
		{Key: "ManagedBy", Value: "orka'i"},
	}
	got := ResolveTags(tags, TagContext{})
	if _, ok := got["Environment"]; ok {
		t.Fatal("expected Environment tag to be skipped when env is empty")
	}
	if got["ManagedBy"] != "orka'i" {
		t.Fatalf("got %q", got["ManagedBy"])
	}
}

func TestResolveTags_SkipUnknownPlaceholder(t *testing.T) {
	tags := []Tag{{Key: "Bad", Value: "{{unknown}}"}}
	got := ResolveTags(tags, TagContext{Env: "prod"})
	if len(got) != 0 {
		t.Fatalf("expected no tags, got %v", got)
	}
}

func TestResolveTags_SkipEmptyKey(t *testing.T) {
	tags := []Tag{{Key: "  ", Value: "value"}}
	got := ResolveTags(tags, TagContext{})
	if len(got) != 0 {
		t.Fatalf("expected no tags, got %v", got)
	}
}

func TestParseAccountTags(t *testing.T) {
	cfg := json.RawMessage(`{"auth_mode":"access_key","tags":[{"key":"ManagedBy","value":"orka'i"}]}`)
	got := ParseAccountTags(cfg)
	if len(got) != 1 || got[0].Key != "ManagedBy" {
		t.Fatalf("got %v", got)
	}
}

func TestParseAccountTags_Empty(t *testing.T) {
	if got := ParseAccountTags(nil); got != nil {
		t.Fatalf("expected nil, got %v", got)
	}
}
