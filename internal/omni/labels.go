package omni

import (
	"errors"
	"io"
	"strings"

	"go.yaml.in/yaml/v4"
)

// TemplateClusterLabels returns the user labels declared for the cluster in a
// cluster template, so they are known before the cluster exists in Omni (or
// before a newly added label has been synced). Both template formats are
// supported:
//
//	kind: Cluster          # omnictl cluster template
//	name: prod-eu
//	labels:
//	  env: production
//
//	metadata:              # Omni resource format
//	  type: Clusters.omni.sidero.dev
//	  labels:
//	    env: production
//
// Omni's own omni.sidero.dev/* labels are dropped. Returns nil when there are
// none or the template cannot be parsed.
func TemplateClusterLabels(content string) map[string]string {
	if !strings.Contains(content, "labels") {
		return nil
	}
	type doc struct {
		Kind     string            `yaml:"kind"`
		Labels   map[string]string `yaml:"labels"`
		Metadata struct {
			Type   string            `yaml:"type"`
			Labels map[string]string `yaml:"labels"`
		} `yaml:"metadata"`
	}
	dec := yaml.NewDecoder(strings.NewReader(content))
	for {
		var d doc
		err := dec.Decode(&d)
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			// Unparseable template: report no labels rather than guessing.
			return nil
		}
		switch {
		case d.Kind == "Cluster":
			return UserLabels(d.Labels)
		case strings.HasPrefix(d.Metadata.Type, "Clusters.omni.sidero.dev"):
			return UserLabels(d.Metadata.Labels)
		}
	}
}
