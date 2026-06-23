package pages

import (
	"encoding/json"
	"regexp"
	"strings"
)

// Tag is a key/value pair configured on a cloud account and applied to AWS
// resources orka'i provisions.
type Tag struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// TagContext holds the dynamic values available when resolving tag placeholders.
type TagContext struct {
	Env     string
	Team    string
	Project string
	Page    string
}

var tagPlaceholder = regexp.MustCompile(`\{\{(\w+)\}\}`)

// ResolveTags substitutes {{env}}, {{team}}, {{project}}, and {{page}} in tag
// values. Tags whose value contains an unknown or empty placeholder are skipped.
// Empty keys are dropped.
func ResolveTags(tags []Tag, tc TagContext) map[string]string {
	out := make(map[string]string, len(tags))
	for _, t := range tags {
		key := strings.TrimSpace(t.Key)
		if key == "" {
			continue
		}
		value, ok := resolveTagValue(t.Value, tc)
		if !ok {
			continue
		}
		out[key] = value
	}
	return out
}

func resolveTagValue(raw string, tc TagContext) (string, bool) {
	var unresolved bool
	resolved := tagPlaceholder.ReplaceAllStringFunc(raw, func(match string) string {
		name := tagPlaceholder.FindStringSubmatch(match)[1]
		var val string
		switch name {
		case "env":
			val = tc.Env
		case "team":
			val = tc.Team
		case "project":
			val = tc.Project
		case "page":
			val = tc.Page
		default:
			unresolved = true
			return match
		}
		if val == "" {
			unresolved = true
		}
		return val
	})
	if unresolved {
		return "", false
	}
	return resolved, true
}

// ParseAccountTags extracts the tags array from a cloud account config JSON.
func ParseAccountTags(cfg json.RawMessage) []Tag {
	if len(cfg) == 0 {
		return nil
	}
	var wrapper struct {
		Tags []Tag `json:"tags"`
	}
	if err := json.Unmarshal(cfg, &wrapper); err != nil {
		return nil
	}
	return wrapper.Tags
}
