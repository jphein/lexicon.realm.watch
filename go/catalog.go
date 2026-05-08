package lexicon

import (
	"bytes"
	"fmt"
	"os"
	"strings"

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

// Bytes serializes the in-memory catalog back to YAML. When a node tree is
// available from LoadCatalog, mutations are applied to that tree so the
// emitted YAML preserves comments, blank lines, scalar styles (`~` vs `""`),
// and key ordering for untouched fields. Without a node tree (e.g., a
// programmatically-constructed Catalog), Bytes falls back to encoding the
// typed slice directly.
func (c *Catalog) Bytes() ([]byte, error) {
	if c.root == nil {
		return c.bytesFromTyped()
	}
	if err := c.applyToNode(); err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(c.root); err != nil {
		return nil, err
	}
	enc.Close()
	return reinsertEntrySeparators(buf.Bytes()), nil
}

// reinsertEntrySeparators puts a blank line back between consecutive
// top-level project entries. yaml.v3 does not preserve blank lines, but the
// curated catalog convention is one blank line between entries for
// readability — this restores that without rewriting the encoder.
func reinsertEntrySeparators(data []byte) []byte {
	lines := strings.Split(string(data), "\n")
	out := make([]string, 0, len(lines)+len(lines)/8)
	for _, line := range lines {
		if strings.HasPrefix(line, "  - id:") && len(out) > 0 {
			prev := out[len(out)-1]
			if prev != "" && prev != "projects:" {
				out = append(out, "")
			}
		}
		out = append(out, line)
	}
	return []byte(strings.Join(out, "\n"))
}

func (c *Catalog) bytesFromTyped() ([]byte, error) {
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

// applyToNode reconciles the in-memory typed Projects slice into the cached
// *yaml.Node tree so a subsequent Encode preserves curated formatting for
// fields the typed model didn't touch. Only fields the catalog API can change
// — current_name and prior_names for renames, plus brand-new project entries
// for appendNew — are written; other fields are left as originally parsed.
func (c *Catalog) applyToNode() error {
	seq, err := findProjectsSequence(c.root)
	if err != nil {
		return err
	}
	idx := map[string]*yaml.Node{}
	for _, item := range seq.Content {
		if item.Kind != yaml.MappingNode {
			continue
		}
		if id := mappingScalarValue(item, "id"); id != "" {
			idx[id] = item
		}
	}
	for _, p := range c.Projects {
		if existing, ok := idx[p.ID]; ok {
			updateProjectMapping(existing, p)
			continue
		}
		seq.Content = append(seq.Content, newProjectMapping(p))
	}
	return nil
}

// findProjectsSequence locates the value node for the top-level `projects:` key.
func findProjectsSequence(root *yaml.Node) (*yaml.Node, error) {
	if root == nil || len(root.Content) == 0 {
		return nil, fmt.Errorf("catalog: empty document")
	}
	top := root.Content[0]
	if top.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("catalog: top-level node is not a mapping")
	}
	for i := 0; i+1 < len(top.Content); i += 2 {
		if top.Content[i].Value == "projects" {
			seq := top.Content[i+1]
			if seq.Kind != yaml.SequenceNode {
				return nil, fmt.Errorf("catalog: `projects` is not a sequence")
			}
			return seq, nil
		}
	}
	return nil, fmt.Errorf("catalog: `projects` key not found")
}

// mappingScalarValue returns the scalar value at key in mapping m, or "".
func mappingScalarValue(m *yaml.Node, key string) string {
	if m == nil || m.Kind != yaml.MappingNode {
		return ""
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1].Value
		}
	}
	return ""
}

// updateProjectMapping writes the small set of fields the typed model can
// have mutated since load. Everything else stays verbatim.
func updateProjectMapping(m *yaml.Node, p *Project) {
	setScalar(m, "current_name", p.CurrentName)
	setPriorNames(m, p.PriorNames)
}

func setScalar(m *yaml.Node, key, value string) {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			v := m.Content[i+1]
			v.Value = value
			if value != "" && v.Tag == "!!null" {
				v.Tag = ""
			}
			return
		}
	}
	m.Content = append(m.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Value: key},
		&yaml.Node{Kind: yaml.ScalarNode, Value: value},
	)
}

// setPriorNames reconciles the prior_names sequence. Append-only renames
// (the typical case) keep existing item nodes verbatim — preserving any
// flow-style entries like `{ name: foo, retired: bar, reason: baz }` — and
// only construct fresh block-style mappings for the new tail.
func setPriorNames(m *yaml.Node, prior []PriorName) {
	var seq *yaml.Node
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == "prior_names" {
			seq = m.Content[i+1]
			break
		}
	}
	if seq == nil {
		m.Content = append(m.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: "prior_names"},
			newPriorNamesSequence(prior),
		)
		return
	}
	if len(prior) == 0 {
		seq.Content = nil
		seq.Style = yaml.FlowStyle
		return
	}
	if len(prior) >= len(seq.Content) {
		// Promote previously empty `[]` flow-style sequence to block on first item.
		if len(seq.Content) == 0 {
			seq.Style = 0
		}
		for i := len(seq.Content); i < len(prior); i++ {
			seq.Content = append(seq.Content, priorNameMapping(prior[i]))
		}
		return
	}
	// Shrink path (rare): rebuild from scratch.
	seq.Content = nil
	seq.Style = 0
	for _, pn := range prior {
		seq.Content = append(seq.Content, priorNameMapping(pn))
	}
}

func newPriorNamesSequence(prior []PriorName) *yaml.Node {
	seq := &yaml.Node{Kind: yaml.SequenceNode}
	if len(prior) == 0 {
		seq.Style = yaml.FlowStyle
		return seq
	}
	for _, pn := range prior {
		seq.Content = append(seq.Content, priorNameMapping(pn))
	}
	return seq
}

func priorNameMapping(pn PriorName) *yaml.Node {
	item := &yaml.Node{Kind: yaml.MappingNode}
	appendKV(item, "name", pn.Name)
	appendKV(item, "retired", pn.Retired)
	appendKV(item, "reason", pn.Reason)
	return item
}

func appendKV(m *yaml.Node, key, value string) {
	m.Content = append(m.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Value: key},
		&yaml.Node{Kind: yaml.ScalarNode, Value: value},
	)
}

// newProjectMapping constructs a fresh entry for an appendNew project. Empty
// optional fields (domain, repo, created) are emitted as `~` to match the
// project's convention for "absent / not yet set" fields.
func newProjectMapping(p *Project) *yaml.Node {
	m := &yaml.Node{Kind: yaml.MappingNode}
	appendKV(m, "id", p.ID)
	appendKV(m, "current_name", p.CurrentName)
	appendKV(m, "kind", p.Kind)
	appendKV(m, "realm", p.Realm)
	appendNullableKV(m, "domain", p.Domain)
	appendNullableKV(m, "repo", p.Repo)
	appendKV(m, "description", p.Description)
	appendNullableKV(m, "created", p.Created)
	m.Content = append(m.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Value: "prior_names"},
		newPriorNamesSequence(p.PriorNames),
	)
	appendKV(m, "status", p.Status)
	if p.Notes != "" {
		appendKV(m, "notes", p.Notes)
	}
	return m
}

func appendNullableKV(m *yaml.Node, key, value string) {
	if value == "" {
		m.Content = append(m.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: key},
			&yaml.Node{Kind: yaml.ScalarNode, Value: "~", Tag: "!!null"},
		)
		return
	}
	appendKV(m, key, value)
}
