package lexicon

import (
	"fmt"
	"os"
	"time"
)

// ClaimOpts describes how to update or create a catalog entry.
//
//   - If RenamesOf is set, the named project (looked up by id) has its
//     CurrentName replaced and the previous CurrentName appended to PriorNames.
//   - If RenamesOf is empty, a new Project entry is created with id = newName.
type ClaimOpts struct {
	RenamesOf   string // existing project id; empty means "new entry"
	Reason      string // attached to the prior_name record (rename mode)
	Retired     string // ISO date for the prior_name record; defaults to today
	Kind        string // for new entries
	Realm       string // for new entries
	Domain      string // for new entries
	Repo        string // for new entries
	Description string // for new entries
	Created     string // for new entries; defaults to today
	Status      string // for new entries; defaults to "active"
}

// Claim either renames an existing project or appends a new one.
func (c *Catalog) Claim(newName string, opts ClaimOpts) error {
	if newName == "" {
		return fmt.Errorf("Claim: newName is empty")
	}
	// Reject if the name is already taken (by id, current_name, or prior_name)
	// by some *other* project. The renamed project's old current_name doesn't
	// count as a collision for itself.
	if existing, ok := c.Resolve(newName); ok {
		if opts.RenamesOf == "" || existing.ID != opts.RenamesOf {
			return fmt.Errorf("name %q is already taken by project %q (current=%q)",
				newName, existing.ID, existing.CurrentName)
		}
	}

	if opts.RenamesOf != "" {
		return c.renameInPlace(newName, opts)
	}
	return c.appendNew(newName, opts)
}

func (c *Catalog) renameInPlace(newName string, opts ClaimOpts) error {
	target, ok := c.Resolve(opts.RenamesOf)
	if !ok {
		return fmt.Errorf("RenamesOf=%q not found in catalog", opts.RenamesOf)
	}
	retired := opts.Retired
	if retired == "" {
		retired = time.Now().UTC().Format("2006-01-02")
	}
	target.PriorNames = append(target.PriorNames, PriorName{
		Name:    target.CurrentName,
		Retired: retired,
		Reason:  opts.Reason,
	})
	target.CurrentName = newName
	return nil
}

func (c *Catalog) appendNew(newName string, opts ClaimOpts) error {
	created := opts.Created
	if created == "" {
		created = time.Now().UTC().Format("2006-01-02")
	}
	status := opts.Status
	if status == "" {
		status = "active"
	}
	c.Projects = append(c.Projects, &Project{
		ID:          newName,
		CurrentName: newName,
		Kind:        opts.Kind,
		Realm:       opts.Realm,
		Domain:      opts.Domain,
		Repo:        opts.Repo,
		Description: opts.Description,
		Created:     created,
		PriorNames:  []PriorName{},
		Status:      status,
	})
	return nil
}

// Save writes the in-memory catalog back to its source path.
func (c *Catalog) Save() error {
	if c.Path == "" {
		return fmt.Errorf("Save: catalog has no source path")
	}
	data, err := c.Bytes()
	if err != nil {
		return err
	}
	return os.WriteFile(c.Path, data, 0o644)
}
