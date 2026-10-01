package setup

func DefaultRegistry() Registry {
	return Registry{
		StepPreflight,
		StepBoard,
		StepAdminWeb,
		StepWifi,
		StepBinaries,
		StepBuzzSeed,
		StepBuzzRelayKey,
		StepBuzzRelay,
		StepBuzzMedia,
		StepAdmind,
		StepCapabilityd,
		StepBlueclawRuntimeBase,
		StepSkills,
		StepBlueclawConfiguration,
		StepBlueclawPayload,
		StepBlueclawPayloadDirect,
		StepOpenRouter,
		StepLocalLLM,
		StepStaging,
		StepBuzzPublicHost,
		StepBuzzChatd,
		StepBuzzMigrate,
		StepServices,
		StepRelay,
		StepUsersSync,
		StepHealth,
	}
}

func JetsonRegistry() Registry {
	return DefaultRegistry()
}
