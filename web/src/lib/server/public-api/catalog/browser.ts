import { z } from 'zod';

import {
  CapabilityAnsweredBy,
  CapabilityEstimatedLatency,
  CapabilityModelVisibility,
  CapabilitySideEffect,
} from './protocol';

import type { CapabilityToolDefinition } from './definition';

const browserFillInputSchema = z.strictObject({
  ref: z.string().optional(),
  selector: z.string().optional(),
  target: z.string().optional(),
  text: z.string(),
});

const browserHandoffInputSchema = z.strictObject({
  message: z.string().optional(),
  url: z.string().optional(),
});

const browserPressInputSchema = z.strictObject({
  key: z.string(),
});

const browserSelectInputSchema = z.strictObject({
  ref: z.string().optional(),
  selector: z.string().optional(),
  target: z.string().optional(),
  value: z.string(),
});

const browserWaitInputSchema = z.strictObject({
  milliseconds: z.int().optional(),
  ref: z.string().optional(),
  selector: z.string().optional(),
  target: z.string().optional(),
});

export const hiddenBrowserToolDefinitions: CapabilityToolDefinition[] = [
  {
    name: "browser_fill",
    namespace: "browser",
    answeredBy: CapabilityAnsweredBy.Local,
    privacyClass: "user_browser",
    policyResource: "tool:browser_fill",
    description: "Fill text into a target in the user's local browser.",
    version: "1",
    estimatedLatency: CapabilityEstimatedLatency.Interactive,
    modelVisibility: CapabilityModelVisibility.Hidden,
    inputSchema: browserFillInputSchema,
    sideEffect: CapabilitySideEffect.ExternalWrite,
    requiresRequesterDevice: true,
    approvalScope: "browser",
  },
  {
    name: "browser_handoff",
    namespace: "browser",
    answeredBy: CapabilityAnsweredBy.Local,
    privacyClass: "user_browser",
    policyResource: "tool:browser_handoff",
    description: "Hand browser control to the user for an interactive step.",
    version: "1",
    estimatedLatency: CapabilityEstimatedLatency.Interactive,
    modelVisibility: CapabilityModelVisibility.Hidden,
    inputSchema: browserHandoffInputSchema,
    sideEffect: CapabilitySideEffect.Connect,
    requiresUserPresence: true,
    requiresRequesterDevice: true,
    requiresCompanionBrowser: true,
    approvalScope: "browser",
  },
  {
    name: "browser_press",
    namespace: "browser",
    answeredBy: CapabilityAnsweredBy.Local,
    privacyClass: "user_browser",
    policyResource: "tool:browser_press",
    description: "Press a key in the user's local browser.",
    version: "1",
    estimatedLatency: CapabilityEstimatedLatency.Interactive,
    modelVisibility: CapabilityModelVisibility.Hidden,
    inputSchema: browserPressInputSchema,
    sideEffect: CapabilitySideEffect.ExternalWrite,
    requiresRequesterDevice: true,
    approvalScope: "browser",
  },
  {
    name: "browser_select",
    namespace: "browser",
    answeredBy: CapabilityAnsweredBy.Local,
    privacyClass: "user_browser",
    policyResource: "tool:browser_select",
    description: "Select a value in the user's local browser.",
    version: "1",
    estimatedLatency: CapabilityEstimatedLatency.Interactive,
    modelVisibility: CapabilityModelVisibility.Hidden,
    inputSchema: browserSelectInputSchema,
    sideEffect: CapabilitySideEffect.ExternalWrite,
    requiresRequesterDevice: true,
    approvalScope: "browser",
  },
  {
    name: "browser_wait",
    namespace: "browser",
    answeredBy: CapabilityAnsweredBy.Local,
    privacyClass: "user_browser",
    policyResource: "tool:browser_wait",
    description: "Wait for a browser target or a bounded interval.",
    version: "1",
    estimatedLatency: CapabilityEstimatedLatency.Interactive,
    modelVisibility: CapabilityModelVisibility.Hidden,
    inputSchema: browserWaitInputSchema,
    sideEffect: CapabilitySideEffect.Read,
    requiresRequesterDevice: true,
    approvalScope: "browser",
  },
];
