package omni

import (
	"maps"
	"testing"
)

func TestUserLabels(t *testing.T) {
	got := UserLabels(map[string]string{
		"env":                     "prod",
		"team":                    "platform",
		"omni.sidero.dev/cluster": "c1",
		"omni.sidero.dev/managed-by-cluster-templates": "",
	})
	if want := map[string]string{"env": "prod", "team": "platform"}; !maps.Equal(got, want) {
		t.Errorf("UserLabels = %v, want %v", got, want)
	}
	if got := UserLabels(map[string]string{"omni.sidero.dev/cluster": "c1"}); got != nil {
		t.Errorf("only system labels should give nil, got %v", got)
	}
	if got := UserLabels(nil); got != nil {
		t.Errorf("nil input should give nil, got %v", got)
	}
}

func TestTemplateClusterLabels(t *testing.T) {
	cases := []struct {
		name, content string
		want          map[string]string
	}{
		{"omnictl template", `kind: Cluster
name: prod-eu
labels:
  env: production
  omni.sidero.dev/system: "x"
kubernetes:
  version: v1.34.1
---
kind: ControlPlane
labels:
  ignored: "yes"
machineClass:
  name: cp
  size: 3
`, map[string]string{"env": "production"}},
		{"resource format", `metadata:
  namespace: default
  type: MachineSets.omni.sidero.dev
  id: prod-eu-workers
  labels:
    omni.sidero.dev/cluster: prod-eu
---
metadata:
  namespace: default
  type: Clusters.omni.sidero.dev
  id: prod-eu
  labels:
    env: production
    region: eu
spec:
  talosversion: 1.11.3
`, map[string]string{"env": "production", "region": "eu"}},
		{"no labels", "kind: Cluster\nname: dev\n", nil},
		{"broken yaml", "kind: Cluster\nlabels: [unclosed\n", nil},
		{"empty", "", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := TemplateClusterLabels(c.content); !maps.Equal(got, c.want) {
				t.Errorf("TemplateClusterLabels = %v, want %v", got, c.want)
			}
		})
	}
}
