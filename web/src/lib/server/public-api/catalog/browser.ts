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

const browserHandoffInputSchema = z.strictObject({
  message: z.string().describe('What the person should do in the browser, in the language of the conversation.').optional(),
  url: z.string().describe('An HTTP or HTTPS address to open in the device browser before handing it over.').optional(),
});

const browserHandoffInputIntentSchema = browserHandoffInputSchema;

const browserHandoffResultSchema = z.strictObject({
  handoffID: resourceIDSchema,
  status: z.literal('waiting'),
  openURL: resourceIDSchema.describe('The intern.kim address where the requester watches and controls the device browser. Share it only once page shows what they need.'),
  expiresAt: resourceIDSchema,
  page: z.strictObject({
    url: resourceIDSchema.describe('The address the device browser actually shows, which may differ from the url asked for.'),
    title: z.string().optional(),
    text: z.string().describe('The start of the text the page shows. Read it: a not-found, error or unexpected page is what the requester would see.').optional(),
  }).describe('What the requester sees when they open openURL.'),
});

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
    name: "browser_handoff",
    namespace: "browser",
    answeredBy: CapabilityAnsweredBy.Local,
    privacyClass: "device_browser",
    policyResource: "tool:browser_handoff",
    description: "Hand the device browser to the requester for a step only a person can do, such as signing in or passing a CAPTCHA. The requester watches and controls it live on intern.kim. Read the returned page first: if it is not the page they need, such as a not-found or error page, fix it with browser_open, since openURL shows whatever the browser shows. Then share openURL, describe the page as it is rather than as you intended, end your turn, and continue when they finish.",
    version: "1",
    estimatedLatency: CapabilityEstimatedLatency.Interactive,
    inputSchema: browserHandoffInputSchema,
    inputIntentSchema: browserHandoffInputIntentSchema,
    result: { schema: browserHandoffResultSchema, effects: [] },
    sideEffect: CapabilitySideEffect.Connect,
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
