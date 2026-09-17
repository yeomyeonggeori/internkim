import { z } from 'zod';

import {
  CapabilityAnsweredBy,
  CapabilityEstimatedLatency,
  CapabilitySideEffect,
} from './protocol';

import type { CapabilityToolDefinition } from './definition';

export const notificationCategories = [
  'message',
  'task',
  'approval',
  'attendance',
  'leave',
  'calendar',
  'mail',
] as const;

export type NotificationCategory = (typeof notificationCategories)[number];

const notificationCategorySchema = z.enum(notificationCategories);

const categoryListDescription =
  'The kinds of thing a person is told about: message for something somebody wrote them, task for work assigned to them, approval for something waiting on their decision, attendance for their own clock, leave for the requests colleagues send, calendar for an event about to start, and mail for a new message in a connected mailbox.';

export const notificationSettingsGetInputSchema = z.strictObject({});

export const notificationSettingsSetInputSchema = z.strictObject({
  turnOn: z.array(notificationCategorySchema).describe(`The categories to start being told about. ${categoryListDescription}`).optional(),
  turnOff: z.array(notificationCategorySchema).describe('The categories to stop being told about. A category named in neither list keeps the setting it has.').optional(),
});

export const notificationSettingsSetInputIntentSchema = notificationSettingsSetInputSchema;

export const notificationSettingsResultSchema = z.strictObject({
  categories: z.array(z.strictObject({
    category: z.string(),
    isOn: z.boolean(),
    isChoosable: z.boolean(),
  })),
  mutedConversationIDs: z.array(z.string()),
});

const conversationIDSchema = z.string()
  .min(1)
  .max(256)
  .describe('The exact id of the conversation, as a message tool answers it. A conversation has no name to approximate, so this is the id itself and nothing else.');

export const conversationMuteInputSchema = z.strictObject({
  conversationID: conversationIDSchema,
});

export const conversationMuteInputIntentSchema = z.strictObject({});

export const conversationMuteResultSchema = z.strictObject({
  conversationID: z.string(),
  isMuted: z.boolean(),
  mutedConversationIDs: z.array(z.string()),
});

export const notificationToolDefinitions: CapabilityToolDefinition[] = [
  {
    name: 'notification_settings_get',
    namespace: 'notification',
    answeredBy: CapabilityAnsweredBy.Record,
    privacyClass: 'user_notification',
    policyResource: 'tool:notification_settings_get',
    description: `What the requester is told about, category by category, and the conversations they have muted. ${categoryListDescription} A category the requester may not choose is answered with isChoosable false, so a change to it would be refused.`,
    version: '1',
    estimatedLatency: CapabilityEstimatedLatency.Low,
    inputSchema: notificationSettingsGetInputSchema,
    result: { schema: notificationSettingsResultSchema, effects: [] },
    sideEffect: CapabilitySideEffect.Read,
  },
  {
    name: 'notification_settings_set',
    namespace: 'notification',
    answeredBy: CapabilityAnsweredBy.Record,
    privacyClass: 'user_notification',
    policyResource: 'tool:notification_settings_set',
    description: 'Change what the requester is told about. Only the categories the call names change, and the whole setting is answered back. A category the requester may not choose is refused by name rather than quietly dropped.',
    version: '1',
    estimatedLatency: CapabilityEstimatedLatency.Medium,
    inputSchema: notificationSettingsSetInputSchema,
    inputIntentSchema: notificationSettingsSetInputIntentSchema,
    result: { schema: notificationSettingsResultSchema, effects: [] },
    sideEffect: CapabilitySideEffect.WorkspaceWrite,
  },
  {
    name: 'conversation_mute',
    namespace: 'notification',
    answeredBy: CapabilityAnsweredBy.Record,
    privacyClass: 'user_notification',
    policyResource: 'tool:conversation_mute',
    description: 'Stop telling the requester about a conversation. The messages keep arriving; the requester is not notified of them. This is the requester\'s own, and it changes nothing for anybody else in the conversation.',
    version: '1',
    estimatedLatency: CapabilityEstimatedLatency.Medium,
    inputSchema: conversationMuteInputSchema,
    inputIntentSchema: conversationMuteInputIntentSchema,
    result: { schema: conversationMuteResultSchema, effects: [] },
    sideEffect: CapabilitySideEffect.WorkspaceWrite,
  },
  {
    name: 'conversation_unmute',
    namespace: 'notification',
    answeredBy: CapabilityAnsweredBy.Record,
    privacyClass: 'user_notification',
    policyResource: 'tool:conversation_unmute',
    description: 'Tell the requester about a conversation again. Read notification_settings_get for the conversations that are muted now.',
    version: '1',
    estimatedLatency: CapabilityEstimatedLatency.Medium,
    inputSchema: conversationMuteInputSchema,
    inputIntentSchema: conversationMuteInputIntentSchema,
    result: { schema: conversationMuteResultSchema, effects: [] },
    sideEffect: CapabilitySideEffect.WorkspaceWrite,
  },
];

export type NotificationSettingsResult = z.infer<typeof notificationSettingsResultSchema>;
export type ConversationMuteResult = z.infer<typeof conversationMuteResultSchema>;
