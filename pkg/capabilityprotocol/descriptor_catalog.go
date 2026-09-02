package capabilityprotocol

// A grant covers a family of tools, and each tool names the family it belongs to.
const (
	BrowserApprovalScope   = "browser"
	FileApprovalScope      = "file"
	DesktopApprovalScope   = "desktop"
	UserInputApprovalScope = "user_input"
)

func mustGeneratedToolDescriptor(name string) Descriptor {
	return MustGeneratedToolDescriptors(name)[0]
}

// Every browser tool drives one browser session, and that session lives on the
// requester's own machine, so the whole family runs there - not just the steps
// that need the person watching.
func companionBrowserDescriptor(name string, options ...func(*Descriptor)) Descriptor {
	descriptor := mustGeneratedToolDescriptor(name)
	descriptor.RequiresRequesterDevice = true
	descriptor.ApprovalScope = BrowserApprovalScope
	for _, option := range options {
		option(&descriptor)
	}
	return descriptor
}

// Some browser steps cannot run headless at all - they need the person's own
// browser window - and the descriptor is where that is written down.
func withCompanionBrowser(descriptor *Descriptor) {
	descriptor.RequiresCompanionBrowser = true
}

func deviceBrowserDescriptor(name string, description string) Descriptor {
	descriptor := mustGeneratedToolDescriptor(name)
	descriptor.Description = description
	descriptor.PrivacyClass = "device_browser"
	descriptor.RequiresUserPresence = false
	return descriptor
}

func CompanionToolDescriptors() []Descriptor {
	return MustCanonicalizeBuiltInDescriptors([]Descriptor{
		companionBrowserDescriptor("browser_open"),
		companionBrowserDescriptor("browser_snapshot"),
		companionBrowserDescriptor("browser_screenshot", withCompanionBrowser),
		NewDescriptor(DescriptorDefinition{
			Identity: DescriptorIdentity{
				Name:            "browser_handoff",
				CanonicalName:   "browser_handoff",
				Namespace:       "browser",
				AnsweredBy:      AnsweredByLocal,
				ModelName:       "browser_handoff",
				ModelVisibility: ModelVisibilityHidden,
			},
			Metadata: DescriptorMetadata{
				Description:              "Hand browser control to the user for an interactive step.",
				Version:                  "1",
				PrivacyClass:             "user_browser",
				EstimatedLatency:         "interactive",
				RequiresUserPresence:     true,
				RequiresRequesterDevice:  true,
				ApprovalScope:            BrowserApprovalScope,
				RequiresCompanionBrowser: true,
				WorksOffline:             false,
				InputSchema:              browserHandoffInputSchema(),
				OutputSchema:             ToolInvokeOutputSchema(),
				PolicyResource:           "tool:browser_handoff",
				SideEffect:               SideEffectConnect,
				Availability:             AvailabilityMetadata{State: AvailabilityOK},
				Idempotency:              IdempotencyMetadata{Scope: "operation"},
			},
		}),
		companionBrowserDescriptor("browser_click"),
		NewDescriptor(DescriptorDefinition{
			Identity: DescriptorIdentity{
				Name:            "browser_fill",
				CanonicalName:   "browser_fill",
				Namespace:       "browser",
				AnsweredBy:      AnsweredByLocal,
				ModelName:       "browser_fill",
				ModelVisibility: ModelVisibilityHidden,
			},
			Metadata: DescriptorMetadata{
				Description:             "Fill text into a target in the user's local browser.",
				Version:                 "1",
				PrivacyClass:            "user_browser",
				EstimatedLatency:        "interactive",
				RequiresUserPresence:    false,
				RequiresRequesterDevice: true,
				ApprovalScope:           BrowserApprovalScope,
				WorksOffline:            false,
				InputSchema:             browserFillInputSchema(),
				OutputSchema:            ToolInvokeOutputSchema(),
				PolicyResource:          "tool:browser_fill",
				SideEffect:              SideEffectExternalWrite,
				Availability:            AvailabilityMetadata{State: AvailabilityOK},
				Idempotency:             IdempotencyMetadata{Scope: "operation"},
			},
		}),
		NewDescriptor(DescriptorDefinition{
			Identity: DescriptorIdentity{
				Name:            "browser_select",
				CanonicalName:   "browser_select",
				Namespace:       "browser",
				AnsweredBy:      AnsweredByLocal,
				ModelName:       "browser_select",
				ModelVisibility: ModelVisibilityHidden,
			},
			Metadata: DescriptorMetadata{
				Description:             "Select a value in the user's local browser.",
				Version:                 "1",
				PrivacyClass:            "user_browser",
				EstimatedLatency:        "interactive",
				RequiresUserPresence:    false,
				RequiresRequesterDevice: true,
				ApprovalScope:           BrowserApprovalScope,
				WorksOffline:            false,
				InputSchema:             browserSelectInputSchema(),
				OutputSchema:            ToolInvokeOutputSchema(),
				PolicyResource:          "tool:browser_select",
				SideEffect:              SideEffectExternalWrite,
				Availability:            AvailabilityMetadata{State: AvailabilityOK},
				Idempotency:             IdempotencyMetadata{Scope: "operation"},
			},
		}),
		NewDescriptor(DescriptorDefinition{
			Identity: DescriptorIdentity{
				Name:            "browser_press",
				CanonicalName:   "browser_press",
				Namespace:       "browser",
				AnsweredBy:      AnsweredByLocal,
				ModelName:       "browser_press",
				ModelVisibility: ModelVisibilityHidden,
			},
			Metadata: DescriptorMetadata{
				Description:             "Press a key in the user's local browser.",
				Version:                 "1",
				PrivacyClass:            "user_browser",
				EstimatedLatency:        "interactive",
				RequiresUserPresence:    false,
				RequiresRequesterDevice: true,
				ApprovalScope:           BrowserApprovalScope,
				WorksOffline:            false,
				InputSchema:             browserPressInputSchema(),
				OutputSchema:            ToolInvokeOutputSchema(),
				PolicyResource:          "tool:browser_press",
				SideEffect:              SideEffectExternalWrite,
				Availability:            AvailabilityMetadata{State: AvailabilityOK},
				Idempotency:             IdempotencyMetadata{Scope: "operation"},
			},
		}),
		NewDescriptor(DescriptorDefinition{
			Identity: DescriptorIdentity{
				Name:            "browser_wait",
				CanonicalName:   "browser_wait",
				Namespace:       "browser",
				AnsweredBy:      AnsweredByLocal,
				ModelName:       "browser_wait",
				ModelVisibility: ModelVisibilityHidden,
			},
			Metadata: DescriptorMetadata{
				Description:             "Wait for a browser target or a bounded interval.",
				Version:                 "1",
				PrivacyClass:            "user_browser",
				EstimatedLatency:        "interactive",
				RequiresUserPresence:    false,
				RequiresRequesterDevice: true,
				ApprovalScope:           BrowserApprovalScope,
				WorksOffline:            false,
				InputSchema:             browserWaitInputSchema(),
				OutputSchema:            ToolInvokeOutputSchema(),
				PolicyResource:          "tool:browser_wait",
				SideEffect:              SideEffectRead,
				Availability:            AvailabilityMetadata{State: AvailabilityOK},
				Idempotency:             IdempotencyMetadata{Scope: "operation"},
			},
		}),
	})
}

func CompanionLLMDescriptors() []Descriptor {
	return MustCanonicalizeBuiltInDescriptors([]Descriptor{
		NewDescriptor(DescriptorDefinition{
			Identity: DescriptorIdentity{
				Name:            "llm_text",
				CanonicalName:   "llm_text",
				Namespace:       "llm",
				AnsweredBy:      AnsweredByLocal,
				ModelName:       "llm_text",
				ModelVisibility: ModelVisibilityHidden,
			},
			Metadata: DescriptorMetadata{
				Description:          "Generate text with the companion's local language model.",
				Version:              "1",
				PrivacyClass:         "model_input",
				EstimatedLatency:     "low",
				RequiresUserPresence: false,
				WorksOffline:         true,
				InputSchema:          TextLLMInputSchema(),
				OutputSchema:         ToolInvokeOutputSchema(),
				PolicyResource:       "tool:llm_text",
				SideEffect:           SideEffectComputation,
				Availability:         AvailabilityMetadata{State: AvailabilityOK},
				Idempotency:          IdempotencyMetadata{Scope: "operation"},
			},
		}),
		NewDescriptor(DescriptorDefinition{
			Identity: DescriptorIdentity{
				Name:            "llm_structured",
				CanonicalName:   "llm_structured",
				Namespace:       "llm",
				AnsweredBy:      AnsweredByLocal,
				ModelName:       "llm_structured",
				ModelVisibility: ModelVisibilityHidden,
			},
			Metadata: DescriptorMetadata{
				Description:          "Generate schema-constrained output with the companion's local language model.",
				Version:              "1",
				PrivacyClass:         "model_input",
				EstimatedLatency:     "low",
				RequiresUserPresence: false,
				WorksOffline:         true,
				InputSchema:          StructuredLLMInputSchema(),
				OutputSchema:         ToolInvokeOutputSchema(),
				PolicyResource:       "tool:llm_structured",
				SideEffect:           SideEffectComputation,
				Availability:         AvailabilityMetadata{State: AvailabilityOK},
				Idempotency:          IdempotencyMetadata{Scope: "operation"},
			},
		}),
		NewDescriptor(DescriptorDefinition{
			Identity: DescriptorIdentity{
				Name:            "embedding_create",
				CanonicalName:   "embedding_create",
				Namespace:       "embedding",
				AnsweredBy:      AnsweredByLocal,
				ModelName:       "embedding_create",
				ModelVisibility: ModelVisibilityHidden,
			},
			Metadata: DescriptorMetadata{
				Description:          "Create embeddings with the companion's local embedding model.",
				Version:              "1",
				PrivacyClass:         "model_input",
				EstimatedLatency:     "low",
				RequiresUserPresence: false,
				WorksOffline:         true,
				InputSchema:          EmbeddingInputSchema(),
				OutputSchema:         ToolInvokeOutputSchema(),
				PolicyResource:       "tool:embedding_create",
				SideEffect:           SideEffectComputation,
				Availability:         AvailabilityMetadata{State: AvailabilityOK},
				Idempotency:          IdempotencyMetadata{Scope: "operation"},
			},
		}),
		NewDescriptor(DescriptorDefinition{
			Identity: DescriptorIdentity{
				Name:            AttentionTriageToolName,
				CanonicalName:   AttentionTriageToolName,
				Namespace:       "attention",
				AnsweredBy:      AnsweredByLocal,
				ModelName:       AttentionTriageToolName,
				ModelVisibility: ModelVisibilityHidden,
			},
			Metadata: DescriptorMetadata{
				Description:          "Classify whether a pending companion job needs remote attention.",
				Version:              "1",
				PrivacyClass:         "model_input",
				EstimatedLatency:     "low",
				RequiresUserPresence: false,
				WorksOffline:         true,
				InputSchema:          attentionTriageInputSchema(),
				OutputSchema:         ToolInvokeOutputSchema(),
				PolicyResource:       "tool:attention.triage",
				SideEffect:           SideEffectComputation,
				Availability:         AvailabilityMetadata{State: AvailabilityOK},
				Idempotency:          IdempotencyMetadata{Scope: "operation"},
			},
		}),
	})
}

func DeviceBrowserDescriptors() []Descriptor {
	return MustCanonicalizeBuiltInDescriptors([]Descriptor{
		deviceBrowserDescriptor("browser_open", "Open an exact HTTP or HTTPS URL in the device browser."),
		deviceBrowserDescriptor("browser_snapshot", "Read the current device browser page structure."),
		deviceBrowserDescriptor("browser_click", "Click one exact target from the current device browser snapshot."),
		NewDescriptor(DescriptorDefinition{
			Identity: DescriptorIdentity{
				Name:            "browser_fill",
				CanonicalName:   "browser_fill",
				Namespace:       "browser",
				AnsweredBy:      AnsweredByLocal,
				ModelName:       "browser_fill",
				ModelVisibility: ModelVisibilityHidden,
			},
			Metadata: DescriptorMetadata{
				Description:             "Fill text into a target in the device browser.",
				Version:                 "1",
				PrivacyClass:            "device_browser",
				EstimatedLatency:        "interactive",
				RequiresUserPresence:    false,
				RequiresRequesterDevice: true,
				ApprovalScope:           BrowserApprovalScope,
				WorksOffline:            false,
				InputSchema:             browserFillInputSchema(),
				OutputSchema:            ToolInvokeOutputSchema(),
				PolicyResource:          "tool:browser_fill",
				SideEffect:              SideEffectExternalWrite,
				Availability:            AvailabilityMetadata{State: AvailabilityOK},
				Idempotency:             IdempotencyMetadata{Scope: "operation"},
			},
		}),
		NewDescriptor(DescriptorDefinition{
			Identity: DescriptorIdentity{
				Name:            "browser_select",
				CanonicalName:   "browser_select",
				Namespace:       "browser",
				AnsweredBy:      AnsweredByLocal,
				ModelName:       "browser_select",
				ModelVisibility: ModelVisibilityHidden,
			},
			Metadata: DescriptorMetadata{
				Description:             "Select a value in the device browser.",
				Version:                 "1",
				PrivacyClass:            "device_browser",
				EstimatedLatency:        "interactive",
				RequiresUserPresence:    false,
				RequiresRequesterDevice: true,
				ApprovalScope:           BrowserApprovalScope,
				WorksOffline:            false,
				InputSchema:             browserSelectInputSchema(),
				OutputSchema:            ToolInvokeOutputSchema(),
				PolicyResource:          "tool:browser_select",
				SideEffect:              SideEffectExternalWrite,
				Availability:            AvailabilityMetadata{State: AvailabilityOK},
				Idempotency:             IdempotencyMetadata{Scope: "operation"},
			},
		}),
		NewDescriptor(DescriptorDefinition{
			Identity: DescriptorIdentity{
				Name:            "browser_press",
				CanonicalName:   "browser_press",
				Namespace:       "browser",
				AnsweredBy:      AnsweredByLocal,
				ModelName:       "browser_press",
				ModelVisibility: ModelVisibilityHidden,
			},
			Metadata: DescriptorMetadata{
				Description:             "Press a key in the device browser.",
				Version:                 "1",
				PrivacyClass:            "device_browser",
				EstimatedLatency:        "interactive",
				RequiresUserPresence:    false,
				RequiresRequesterDevice: true,
				ApprovalScope:           BrowserApprovalScope,
				WorksOffline:            false,
				InputSchema:             browserPressInputSchema(),
				OutputSchema:            ToolInvokeOutputSchema(),
				PolicyResource:          "tool:browser_press",
				SideEffect:              SideEffectExternalWrite,
				Availability:            AvailabilityMetadata{State: AvailabilityOK},
				Idempotency:             IdempotencyMetadata{Scope: "operation"},
			},
		}),
		NewDescriptor(DescriptorDefinition{
			Identity: DescriptorIdentity{
				Name:            "browser_wait",
				CanonicalName:   "browser_wait",
				Namespace:       "browser",
				AnsweredBy:      AnsweredByLocal,
				ModelName:       "browser_wait",
				ModelVisibility: ModelVisibilityHidden,
			},
			Metadata: DescriptorMetadata{
				Description:             "Wait for a browser target or a bounded interval.",
				Version:                 "1",
				PrivacyClass:            "device_browser",
				EstimatedLatency:        "interactive",
				RequiresUserPresence:    false,
				RequiresRequesterDevice: true,
				ApprovalScope:           BrowserApprovalScope,
				WorksOffline:            false,
				InputSchema:             browserWaitInputSchema(),
				OutputSchema:            ToolInvokeOutputSchema(),
				PolicyResource:          "tool:browser_wait",
				SideEffect:              SideEffectRead,
				Availability:            AvailabilityMetadata{State: AvailabilityOK},
				Idempotency:             IdempotencyMetadata{Scope: "operation"},
			},
		}),
	})
}
