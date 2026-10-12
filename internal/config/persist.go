package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// SetProcessAutostart rewrites autostart for one process in the file that
// defines it, preserving all other content (including comments) via a
// yaml.Node round-trip. This is what the API's enable/disable endpoints
// use to persist the change.
func SetProcessAutostart(path, name string, enabled bool) error {
	return setNestedBool(path, "processes", name, "autostart", enabled)
}

// SetProcessAutorestart rewrites the restart policy for one process
// ("true", "false" or "unexpected"), with the same round-trip guarantees.
func SetProcessAutorestart(path, name, value string) error {
	if value != "true" && value != "false" && value != "unexpected" {
		return fmt.Errorf("autorestart must be true, false, or unexpected (got %q)", value)
	}
	return setNestedValue(path, "processes", name, "autorestart",
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value})
}

// SetScheduleEnabled rewrites enabled for one schedule, same guarantees.
func SetScheduleEnabled(path, name string, enabled bool) error {
	return setNestedBool(path, "schedules", name, "enabled", enabled)
}

// setNestedBool rewrites <section>.<name>.<key> = <bool> in a YAML file,
// preserving everything else (including comments).
func setNestedBool(path, section, name, key string, value bool) error {
	return setNestedValue(path, section, name, key,
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: fmt.Sprintf("%t", value)})
}

// setNestedValue rewrites <section>.<name>.<key> to an arbitrary scalar node.
func setNestedValue(path, section, name, key string, val *yaml.Node) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	if len(doc.Content) == 0 {
		return fmt.Errorf("%s is empty", path)
	}
	root := doc.Content[0]

	sec := mapValue(root, section)
	if sec == nil {
		return fmt.Errorf("%s has no %s section", path, section)
	}
	entry := mapValue(sec, name)
	if entry == nil {
		return fmt.Errorf("%q not defined under %s in %s", name, section, path)
	}
	setMapValue(entry, key, val)

	out, err := yaml.Marshal(&doc)
	if err != nil {
		return fmt.Errorf("encode %s: %w", path, err)
	}
	if err := os.WriteFile(path, out, 0644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// mapValue returns the value node for key in a yaml mapping node.
func mapValue(mapping *yaml.Node, key string) *yaml.Node {
	if mapping == nil || mapping.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			return mapping.Content[i+1]
		}
	}
	return nil
}

// setMapValue sets an existing key to val, or appends it when missing.
func setMapValue(mapping *yaml.Node, key string, val *yaml.Node) {
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			mapping.Content[i+1] = val
			return
		}
	}
	mapping.Content = append(mapping.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key},
		val,
	)
}
