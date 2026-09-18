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

const browserFillInputSchema = z.strictObject({
  ref: z.string().optional(),
  selector: z.string().optional(),
  target: z.string().optional(),
  text: z.string(),
});

const browserFillInputIntentSchema = browserFillInputSchema.partial();

const browserPressInputSchema = z.strictObject({
  key: z.string(),
});

const browserPressInputIntentSchema = browserPressInputSchema.partial();

const browserSelectInputSchema = z.strictObject({
  ref: z.string().optional(),
  selector: z.string().optional(),
  target: z.string().optional(),
  value: z.string(),
});

const browserSelectInputIntentSchema = browserSelectInputSchema.partial();

const browserWaitInputSchema = z.strictObject({
  milliseconds: z.int().optional(),
  ref: z.string().optional(),
  selector: z.string().optional(),
  target: z.string().optional(),
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
    name: "browser_fill",
    namespace: "browser",
    answeredBy: CapabilityAnsweredBy.Local,
    privacyClass: "user_browser",
    policyResource: "tool:browser_fill",
    description: "Fill text into a target in the user's local browser.",
    version: "1",
    estimatedLatency: CapabilityEstimatedLatency.Interactive,
    inputSchema: browserFillInputSchema,
    inputIntentSchema: browserFillInputIntentSchema,
    result: { schema: browserFillResultSchema, effects: [] },
    sideEffect: CapabilitySideEffect.ExternalWrite,
    requiresRequesterDevice: true,
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
    inputSchema: browserPressInputSchema,
    inputIntentSchema: browserPressInputIntentSchema,
    result: { schema: browserPressResultSchema, effects: [] },
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
    inputSchema: browserSelectInputSchema,
    inputIntentSchema: browserSelectInputIntentSchema,
    result: { schema: browserSelectResultSchema, effects: [] },
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
    inputSchema: browserWaitInputSchema,
    result: { schema: browserWaitResultSchema, effects: [] },
    sideEffect: CapabilitySideEffect.Read,
    requiresRequesterDevice: true,
    approvalScope: "browser",
  },
];
