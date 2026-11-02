package nativerouting

// ProfileSpec is one configured native profile. The repository config is the
// only source of these values; adapters launch the model and effort named here
// and do not keep a second roster.
type ProfileSpec struct {
	ID        string
	Model     string
	Effort    string
	Provides  Provides
	CostRank  int
	Strongest bool
}

// KnownLaunch reports whether a configured profile names this model and effort.
func KnownLaunch(profiles map[Role][]ProfileSpec, model, effort string) bool {
	for _, specs := range profiles {
		for _, spec := range specs {
			if spec.Model == model && spec.Effort == effort {
				return true
			}
		}
	}
	return false
}

// CloneProfiles returns a deep-enough copy that a caller cannot mutate the
// stored roster through the map or slice it passed in.
func CloneProfiles(in map[Role][]ProfileSpec) map[Role][]ProfileSpec {
	if len(in) == 0 {
		return nil
	}
	out := make(map[Role][]ProfileSpec, len(in))
	for role, specs := range in {
		out[role] = append([]ProfileSpec(nil), specs...)
	}
	return out
}
