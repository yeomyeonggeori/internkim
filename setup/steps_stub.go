package setup

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

func (registry Registry) WithOverride(name string, replacement Step) Registry {
	overridden := make(Registry, len(registry))
	for index, step := range registry {
		if step.Name == name {
			if replacement.Name == "" {
				replacement.Name = step.Name
			}
			if replacement.Deps == nil {
				replacement.Deps = step.Deps
			}
			overridden[index] = replacement
		} else {
			overridden[index] = step
		}
	}
	return overridden
}
