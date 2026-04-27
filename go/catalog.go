package lexicon

import (
	"bytes"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Project is one entry in catalog/projects.yaml.
type Project struct {
	ID          string      `yaml:"id"`
	CurrentName string      `yaml:"current_name"`
	Kind        string      `yaml:"kind"`
	Realm       string      `yaml:"realm"`
	Domain      string      `yaml:"domain"`
	Repo        string      `yaml:"repo"`
	Description string      `yaml:"description"`
	Created     string      `yaml:"created"`
	PriorNames  []PriorName `yaml:"prior_names"`
	Status      string      `yaml:"status"`
	Notes       string      `yaml:"notes,omitempty"`
}

// PriorName records a single rename event in a project's history.
type PriorName struct {
	Name    string `yaml:"name"`
	Retired string `yaml:"retired"`
	Reason  string `yaml:"reason"`
}

// Catalog holds the parsed catalog file along with the original YAML node tree
// (used by Save to round-trip with comments / order preserved).
type Catalog struct {
	Path     string
	Projects []*Project
	root     *yaml.Node // the full document, retained for round-tripping
}

// LoadCatalog reads a projects.yaml file.
func LoadCatalog(path string) (*Catalog, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	type wire struct {
		Projects []*Project `yaml:"projects"`
	}
	var w wire
	if err := root.Decode(&w); err != nil {
		return nil, fmt.Errorf("decode %s: %w", path, err)
	}
	return &Catalog{Path: path, Projects: w.Projects, root: &root}, nil
}

// Bytes serializes the in-memory Projects back to YAML. Round-trip fidelity
// (comments, order) for v0.1 is best-effort: this re-emits from the typed
// slice, accepting that hand-written comments may not survive a Save call.
// Catalogs in v0.1 are short (≤30 entries) — formatting drift is tolerable.
//
// If a more exact round-trip is needed, Task 11 swaps this for a node-tree
// edit that mutates only the changed entry.
func (c *Catalog) Bytes() ([]byte, error) {
	wrap := struct {
		Projects []*Project `yaml:"projects"`
	}{Projects: c.Projects}
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(wrap); err != nil {
		return nil, err
	}
	enc.Close()
	return buf.Bytes(), nil
}
