package nativerouting

import "fmt"

// SelectExact returns only the named native pair from this validated role
// catalog. Unlike semantic resolution, it never promotes or substitutes.
func (c ValidatedCatalog) SelectExact(model, effort string) (SelectedProfile, error) {
	if c.Identity == "" {
		return SelectedProfile{}, fmt.Errorf("native route requires a validated catalog")
	}
	var found *CatalogEntry
	for _, entry := range c.Entries() {
		if entry.NativeModel != model || entry.NativeEffort != effort {
			continue
		}
		if found != nil {
			return SelectedProfile{}, fmt.Errorf("ambiguous native pair %q/%q", model, effort)
		}
		copy := entry
		found = &copy
	}
	if found == nil {
		return SelectedProfile{}, fmt.Errorf("%s/%s does not support native pair %q/%q", c.Catalog.Harness, c.Catalog.Role, model, effort)
	}
	return selectedProfile(c, *found), nil
}
