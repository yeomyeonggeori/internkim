import { z } from 'zod';

import {
  CapabilityAnsweredBy,
  CapabilityEstimatedLatency,
  CapabilitySideEffect,
} from './protocol';

import type { CapabilityToolDefinition } from './definition';

const computerTaskInputValueSchema = z.strictObject({
  name: z.string().min(1).describe('What the text is for, such as "search query" or "message".'),
  text: z.string().describe('The exact text to type.'),
});

const computerTaskInputSchema = z.strictObject({
  goal: z.string().min(1).describe('What must be true when the task is done, said as an outcome the requester would recognise on screen, not as a list of clicks.'),
  startURL: z.string().describe('An HTTP or HTTPS address to open first. Leave it out to continue from the page the companion browser already shows.').optional(),
  inputs: z.array(computerTaskInputValueSchema).max(16).describe('Every piece of text the task may type. Nothing outside this list is ever typed.').optional(),
  maxSteps: z.int().min(1).max(40).describe('How many actions the task may take before it gives up. Defaults to 12.').optional(),
});

const computerTaskInputIntentSchema = computerTaskInputSchema.partial();

const computerTaskStepSchema = z.strictObject({
  step: z.int(),
  candidateID: z.string(),
  description: z.string(),
  confidence: z.number(),
});

const computerTaskResultSchema = z.strictObject({
  outcome: z.enum(['verified', 'refuted', 'unknown', 'abstained', 'budget_exhausted']).describe('verified: a fresh observation showed the goal reached. refuted: the page showed the goal cannot be reached. unknown: an action failed and its effect is unknown. abstained: no safe step remained. budget_exhausted: maxSteps ran out.'),
  summary: z.string().describe('What happened, in one or two sentences a person would understand.'),
  steps: z.array(computerTaskStepSchema),
  page: z.strictObject({
    url: z.string(),
    title: z.string(),
    text: z.string().describe('The start of the text the page showed when the task ended.'),
  }).describe('The page as it was when the task ended, so the caller can judge the result itself.'),
});

const computerConnectInputSchema = z.strictObject({});

const computerConnectResultSchema = z.strictObject({
  code: z.string().describe('A pairing code that works for ten minutes and belongs to the requester alone.'),
  expiresAt: z.string().describe('When the code stops working, as an RFC 3339 timestamp.'),
  installCommand: z.string().describe('Installs or updates the companion binary. Skip it when the requester already has the companion installed.'),
  pairCommand: z.string().describe("Pairs the companion on the requester's computer with this company computer, code included."),
  serviceCommand: z.string().describe('Keeps the companion running in the background after pairing.'),
  documentationURL: z.string().describe('The companion page in the documentation.'),
});

export const computerToolDefinitions: CapabilityToolDefinition[] = [
  {
    name: "computer_connect",
    namespace: "computer",
    answeredBy: CapabilityAnsweredBy.Company,
    privacyClass: "user_browser",
    policyResource: "tool:computer_connect",
    description: "Issue a pairing code so the requester can connect the companion on their own computer. Call it when computer_task or a browser tool was denied with not_connected, or when the requester asks to connect their computer. Give the requester the commands to run in order: installCommand (skip it when the companion is already installed), pairCommand, then serviceCommand. The code is the requester's alone: send it in a direct message, never in a channel.",
    version: "1",
    estimatedLatency: CapabilityEstimatedLatency.Low,
    inputSchema: computerConnectInputSchema,
    inputIntentSchema: computerConnectInputSchema,
    result: { schema: computerConnectResultSchema, effects: [] },
    sideEffect: CapabilitySideEffect.Connect,
    requiresUserPresence: true,
  },
  {
    name: "computer_task",
    namespace: "computer",
    answeredBy: CapabilityAnsweredBy.Local,
    privacyClass: "user_browser",
    policyResource: "tool:computer_task",
    description: "Carry out a goal in the requester's own companion browser. A decision model reads the page, picks one safe action at a time from what is actually on screen, and stops when the goal is reached, cannot be reached, or no safe step remains. State the goal as the outcome to reach and put any text to type in inputs. The browser is a persistent profile of the companion's own, so sign-ins made there stay for later tasks but the requester's personal browser is never touched. Read the returned page and outcome before telling the requester what was done.",
    version: "1",
    estimatedLatency: CapabilityEstimatedLatency.Interactive,
    inputSchema: computerTaskInputSchema,
    inputIntentSchema: computerTaskInputIntentSchema,
    result: { schema: computerTaskResultSchema, effects: [] },
    sideEffect: CapabilitySideEffect.ExternalWrite,
    requiresRequesterDevice: true,
    requiresCompanionBrowser: true,
    approvalScope: "desktop",
  },
];
