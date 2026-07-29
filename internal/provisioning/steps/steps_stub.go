package setup

func DefaultRegistry() Registry {
	return Registry{
		StepPreflight,
		StepBoard,
		StepAdminWeb,
		StepCloudflareAccess,
		StepWifi,
		StepBinaries,
		StepAdmind,
		StepCapabilityd,
		StepBlueclawRuntimeBase,
		StepSkills,
		StepBlueclawConfiguration,
		StepBlueclawPayload,
		StepBlueclawPayloadDirect,
		StepOpenRouter,
		StepBuzzSeed,
		StepBuzzRelayKey,
		StepLocalLLM,
		StepTunnel,
		StepGoogle,
		StepStaging,
		StepMattermost,
		StepBuzzRelay,
		StepBuzzMedia,
		StepSlackToken,
		StepServices,
		StepUsersSync,
		StepHealth,
	}
}

func JetsonRegistry() Registry {
	return DefaultRegistry()
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
