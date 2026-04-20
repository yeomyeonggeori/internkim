package setup

// Placeholder steps. Their Run bodies will be migrated out of main.runSetup in
// follow-up commits. Until then, the main package passes concrete Run/
// IsSatisfied closures via NewRegistry so the pipeline still works.

// Until these are migrated out of main.runSetup, their IsSatisfied returns
// true — so auto-dep resolution from a migrated step (e.g. google) does not
// try to re-run them. Forcing a rerun of a non-migrated step via
// --only/--force currently has no effect; migrate the step first.
var stubSatisfied = func(_ *Context) bool { return true }

var (
	StepBoard      = Step{Name: "board", IsSatisfied: stubSatisfied}
	StepWifi       = Step{Name: "wifi", Deps: []string{"board"}, IsSatisfied: stubSatisfied}
	StepBinaries   = Step{Name: "binaries", Deps: []string{"board"}, IsSatisfied: stubSatisfied}
	StepSkills     = Step{Name: "skills", Deps: []string{"binaries"}, IsSatisfied: stubSatisfied}
	StepOpenRouter = Step{Name: "openrouter", Deps: []string{"binaries"}, IsSatisfied: stubSatisfied}
	StepTunnel     = Step{Name: "tunnel", Deps: []string{"binaries", "openrouter"}, IsSatisfied: stubSatisfied}
	StepMattermost = Step{Name: "mattermost", Deps: []string{"binaries", "tunnel"}, IsSatisfied: stubSatisfied}
	StepServices   = Step{Name: "services", Deps: []string{"binaries", "openrouter", "mattermost"}, IsSatisfied: stubSatisfied}
)

// DefaultRegistry returns the step pipeline in execution order.
// The main package wraps each entry with concrete Run/IsSatisfied closures.
func DefaultRegistry() Registry {
	return Registry{
		StepBoard,
		StepWifi,
		StepBinaries,
		StepSkills,
		StepOpenRouter,
		StepTunnel,
		StepGoogle,
		StepMattermost,
		StepServices,
	}
}

// WithOverride returns a copy of r with step `name` replaced by `replacement`.
// Useful for the main package to inject Run/IsSatisfied bodies for the
// not-yet-migrated steps while keeping Deps/Name from the registry.
func (r Registry) WithOverride(name string, replacement Step) Registry {
	out := make(Registry, len(r))
	for i, s := range r {
		if s.Name == name {
			// Preserve Deps if replacement doesn't override them.
			if replacement.Name == "" {
				replacement.Name = s.Name
			}
			if replacement.Deps == nil {
				replacement.Deps = s.Deps
			}
			out[i] = replacement
		} else {
			out[i] = s
		}
	}
	return out
}
