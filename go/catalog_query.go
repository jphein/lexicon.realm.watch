package lexicon

// Resolve looks up a project by id, current_name, or any prior name. Returns
// the matched project (still pointing at its current name) and ok=true on hit.
//
// Lookup order: id → current_name → prior_names. The first match wins, so a
// project whose id collides with another's prior_name resolves to the id holder.
func (c *Catalog) Resolve(name string) (*Project, bool) {
	// First pass: id match.
	for _, p := range c.Projects {
		if p.ID == name {
			return p, true
		}
	}
	// Second pass: current_name match.
	for _, p := range c.Projects {
		if p.CurrentName == name {
			return p, true
		}
	}
	// Third pass: any prior_name match.
	for _, p := range c.Projects {
		for _, pn := range p.PriorNames {
			if pn.Name == name {
				return p, true
			}
		}
	}
	return nil, false
}

// ByRealm returns all projects in the named realm.
func (c *Catalog) ByRealm(realm string) []*Project {
	out := []*Project{}
	for _, p := range c.Projects {
		if p.Realm == realm {
			out = append(out, p)
		}
	}
	return out
}

// ByKind returns all projects of the named kind.
func (c *Catalog) ByKind(kind string) []*Project {
	out := []*Project{}
	for _, p := range c.Projects {
		if p.Kind == kind {
			out = append(out, p)
		}
	}
	return out
}

// ByStatus returns all projects in the given status.
func (c *Catalog) ByStatus(status string) []*Project {
	out := []*Project{}
	for _, p := range c.Projects {
		if p.Status == status {
			out = append(out, p)
		}
	}
	return out
}
