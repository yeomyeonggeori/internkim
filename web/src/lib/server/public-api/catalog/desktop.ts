import { z } from 'zod';

import {
  CapabilityAnsweredBy,
  CapabilityEstimatedLatency,
  CapabilitySideEffect,
} from './protocol';

import type { CapabilityToolDefinition } from './definition';

export const desktopWorkTimeoutSecond = 120;

export const desktopPrograms = ['agent-desktop', 'agent-browser'] as const;

const desktopProgramSchema = z.enum(desktopPrograms)
  .describe('agent-desktop (the default) operates apps and windows. agent-browser operates web pages in a browser window of its own on that computer, which starts signed in to nothing.')
  .optional();

const desktopReadInputSchema = z.strictObject({
  program: desktopProgramSchema,
  arguments: z.array(z.string())
    .min(1)
    .describe('One command line of the program without its name, one word per entry: ["snapshot", "--app", "Finder"]. The first entry is the command.'),
});

const desktopActInputSchema = z.strictObject({
  program: desktopProgramSchema,
  arguments: z.array(z.string())
    .min(1)
    .describe('One command line of the program without its name, one word per entry: ["click", "@s8f3k2p9:e4"]. The first entry is the command; a ref is copied exactly from the latest snapshot.'),
  text: z.string()
    .describe('The text a type, set-value, clipboard-set or fill command writes. Give it here and leave it out of arguments.')
    .optional(),
});

const desktopActInputIntentSchema = desktopActInputSchema.partial();

const desktopAttachmentSchema = z.strictObject({
  filename: z.string().min(1),
  contentType: z.string().min(1),
  sizeBytes: z.number().int().nonnegative(),
  contentBase64: z.string().min(1),
});

const desktopResultSchema = z.strictObject({
  output: z.string().describe("The program's JSON envelope for the command, as it printed it. Under a snapshot the host asked for again at a smaller depth, a note says so. Under a delivered act, the snapshot the host then took of the window or page it left."),
  attachments: z.array(desktopAttachmentSchema)
    .describe('The picture a screenshot command took. The envelope then carries its size without the picture.')
    .optional(),
});

export const desktopToolDefinitions: CapabilityToolDefinition[] = [
  {
    name: 'desktop_read',
    namespace: 'desktop',
    answeredBy: CapabilityAnsweredBy.Local,
    privacyClass: 'requester_desktop',
    policyResource: 'tool:desktop_read',
    description: "Read the requester's own computer through agent-desktop, which runs inside the internkim desktop app they have open: running apps and windows, an app's accessibility tree with refs, one element's state, a screenshot. A snapshot that names no size is taken interactive and compact. It reaches only the computer of the person who asked, never a colleague's and never the company computer. A call whose arguments the program refuses comes back with that command's usage. [\"skills\", \"get\", \"desktop\"] prints the program's guide to refs, retries and headless input. For a web page use program agent-browser, whose guide is [\"skills\", \"get\", \"core\"]: desktop_act [\"open\", url] opens the page in a browser window of its own, and [\"snapshot\"] reads it.",
    version: '1',
    estimatedLatency: CapabilityEstimatedLatency.Interactive,
    timeoutSecond: desktopWorkTimeoutSecond,
    requiresUserPresence: true,
    inputSchema: desktopReadInputSchema,
    result: { schema: desktopResultSchema, effects: [] },
    sideEffect: CapabilitySideEffect.Read,
  },
  {
    name: 'desktop_act',
    namespace: 'desktop',
    answeredBy: CapabilityAnsweredBy.Local,
    privacyClass: 'requester_desktop',
    policyResource: 'tool:desktop_act',
    description: "Operate the requester's own computer through agent-desktop: click, type into or set a ref, press keys, launch or close an app, move a window, write the clipboard. It also runs every desktop_read command, snapshot included. The requester sees it happen on their screen. A delivered act answers with a new snapshot of the window or page it left, and the next ref comes from that snapshot, because refs belong to the snapshot that printed them. A call whose arguments the program refuses comes back with that command's usage. Never type a password or a one-time code; ask the requester to enter it themselves.",
    version: '1',
    estimatedLatency: CapabilityEstimatedLatency.Interactive,
    timeoutSecond: desktopWorkTimeoutSecond,
    requiresUserPresence: true,
    inputSchema: desktopActInputSchema,
    inputIntentSchema: desktopActInputIntentSchema,
    result: { schema: desktopResultSchema, effects: [] },
    sideEffect: CapabilitySideEffect.ExternalWrite,
  },
];
