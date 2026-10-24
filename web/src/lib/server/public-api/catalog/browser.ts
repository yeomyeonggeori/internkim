import { z } from 'zod';

import {
  CapabilityAnsweredBy,
  CapabilityEstimatedLatency,
  CapabilitySideEffect,
} from './protocol';

import type { CapabilityToolDefinition } from './definition';

const resourceIDSchema = z.string()
  .min(1)
  .regex(/^\S(?:.*\S)?$/, 'Resource identity must not have leading or trailing whitespace.');

export const browserWithheldRequestSchema = z.string()
  .describe('Leave unset. The runtime sets it when the requester approves a request the page tried to send.')
  .optional();

const browserFillInputSchema = z.strictObject({
  ref: z.string().optional(),
  selector: z.string().optional(),
  target: z.string().optional(),
  text: z.string(),
  withheldRequest: browserWithheldRequestSchema,
});

const browserFillInputIntentSchema = browserFillInputSchema.partial();

const browserPressInputSchema = z.strictObject({
  key: z.string(),
  withheldRequest: browserWithheldRequestSchema,
});

const browserPressInputIntentSchema = browserPressInputSchema.partial();

const browserSelectInputSchema = z.strictObject({
  ref: z.string().optional(),
  selector: z.string().optional(),
  target: z.string().optional(),
  value: z.string(),
  withheldRequest: browserWithheldRequestSchema,
});

const browserSelectInputIntentSchema = browserSelectInputSchema.partial();

const browserWaitInputSchema = z.strictObject({
  milliseconds: z.int().optional(),
  ref: z.string().optional(),
  selector: z.string().optional(),
  target: z.string().optional(),
  withheldRequest: browserWithheldRequestSchema,
});

const browserHandoffInputSchema = z.strictObject({
  step: z.string()
    .min(1)
    .describe('What the requester does in the browser themselves, in a few plain words: "sign in to the vendor portal", "enter the code sent to their phone".'),
  handoff: z.string()
    .describe('Leave unset. The runtime sets it when the requester takes the browser.')
    .optional(),
});

const browserHandoffResultSchema = z.strictObject({
  ended: z.enum(['done', 'abandoned', 'expired'])
    .describe('done: the requester handed the browser back as finished. abandoned: they handed it back without finishing. expired: nobody handed it back in time.'),
  origin: z.string().describe('The origin of the page the browser shows now.'),
  signInOrigin: z.string().describe('The origin of the page that was handed over.'),
  signInCookiesChanged: z.boolean()
    .describe("Whether this browser's cookies for signInOrigin changed while the requester had it. Absent when that could not be compared.")
    .optional(),
});

const browserFillResultSchema = z.strictObject({
  ok: z.literal(true),
  action: z.literal('fill'),
  target: resourceIDSchema,
  capturedAt: resourceIDSchema,
});

const browserSelectResultSchema = z.strictObject({
  ok: z.literal(true),
  action: z.literal('select'),
  target: resourceIDSchema,
  capturedAt: resourceIDSchema,
});

const browserPressResultSchema = z.strictObject({
  ok: z.literal(true),
  action: z.literal('press'),
  capturedAt: resourceIDSchema,
});

const browserWaitResultSchema = z.strictObject({
  ok: z.literal(true),
  action: z.literal('wait'),
  target: resourceIDSchema.describe('The target that was waited for. Absent when the wait was a bounded interval.').optional(),
  capturedAt: resourceIDSchema,
});

export const browserControlToolDefinitions: CapabilityToolDefinition[] = [
  {
    name: "browser_handoff",
    namespace: "browser",
    answeredBy: CapabilityAnsweredBy.Local,
    privacyClass: "device_browser",
    policyResource: "tool:browser_handoff",
    description: "Hand the browser to the requester so they do one step in it themselves, such as signing in or entering a one-time code. Open the page that needs them first. Never type a password or code yourself and never ask for one in the conversation. The call waits while they work and answers how it ended and where the browser is; nothing they see or type there reaches you.",
    version: "1",
    estimatedLatency: CapabilityEstimatedLatency.Interactive,
    inputSchema: browserHandoffInputSchema,
    result: { schema: browserHandoffResultSchema, effects: [] },
    sideEffect: CapabilitySideEffect.Read,
  },
  {
    name: "browser_fill",
    namespace: "browser",
    answeredBy: CapabilityAnsweredBy.Local,
    privacyClass: "device_browser",
    policyResource: "tool:browser_fill",
    description: "Fill text into a target on the current browser page.",
    version: "1",
    estimatedLatency: CapabilityEstimatedLatency.Interactive,
    inputSchema: browserFillInputSchema,
    inputIntentSchema: browserFillInputIntentSchema,
    result: { schema: browserFillResultSchema, effects: [] },
    sideEffect: CapabilitySideEffect.ExternalWrite,
  },
  {
    name: "browser_press",
    namespace: "browser",
    answeredBy: CapabilityAnsweredBy.Local,
    privacyClass: "device_browser",
    policyResource: "tool:browser_press",
    description: "Press a key on the current browser page.",
    version: "1",
    estimatedLatency: CapabilityEstimatedLatency.Interactive,
    inputSchema: browserPressInputSchema,
    inputIntentSchema: browserPressInputIntentSchema,
    result: { schema: browserPressResultSchema, effects: [] },
    sideEffect: CapabilitySideEffect.ExternalWrite,
  },
  {
    name: "browser_select",
    namespace: "browser",
    answeredBy: CapabilityAnsweredBy.Local,
    privacyClass: "device_browser",
    policyResource: "tool:browser_select",
    description: "Select a value in a target on the current browser page.",
    version: "1",
    estimatedLatency: CapabilityEstimatedLatency.Interactive,
    inputSchema: browserSelectInputSchema,
    inputIntentSchema: browserSelectInputIntentSchema,
    result: { schema: browserSelectResultSchema, effects: [] },
    sideEffect: CapabilitySideEffect.ExternalWrite,
  },
  {
    name: "browser_wait",
    namespace: "browser",
    answeredBy: CapabilityAnsweredBy.Local,
    privacyClass: "device_browser",
    policyResource: "tool:browser_wait",
    description: "Wait for a browser target or a bounded interval.",
    version: "1",
    estimatedLatency: CapabilityEstimatedLatency.Interactive,
    inputSchema: browserWaitInputSchema,
    result: { schema: browserWaitResultSchema, effects: [] },
    sideEffect: CapabilitySideEffect.Read,
  },
];
