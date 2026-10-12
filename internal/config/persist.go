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

// SetScheduleEnabled rewrites enabled for one schedule, same guarantees.
func SetScheduleEnabled(path, name string, enabled bool) error {
	return setNestedBool(path, "schedules", name, "enabled", enabled)
}

// setNestedBool rewrites <section>.<name>.<key> = <bool> in a YAML file,
// preserving everything else (including comments).
func setNestedBool(path, section, name, key string, value bool) error {
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
	if err := setMapBool(entry, key, value); err != nil {
		return fmt.Errorf("update %s: %w", path, err)
	}

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

// setMapBool sets an existing key, or appends it when missing.
func setMapBool(mapping *yaml.Node, key string, value bool) error {
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			mapping.Content[i+1].Kind = yaml.ScalarNode
			mapping.Content[i+1].Tag = "!!bool"
			mapping.Content[i+1].Value = fmt.Sprintf("%t", value)
			return nil
		}
	}
	mapping.Content = append(mapping.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key},
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: fmt.Sprintf("%t", value)},
	)
	return nil
}
