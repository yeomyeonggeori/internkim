import { z } from 'zod';

import {
  CapabilityAnsweredBy,
  CapabilityEstimatedLatency,
  CapabilityModelVisibility,
  CapabilitySideEffect,
  ResourceEffectIdentity,
} from './protocol';
import {
  ResourceMutationEffect,
  type CapabilityResultDefinition,
  type CapabilityToolDefinition,
} from './definition';

export enum ChannelOrderToolName {
  Get = 'channel_order_get',
  Set = 'channel_order_set',
  Pin = 'channel_pin',
  Unpin = 'channel_unpin',
}

const channelIDSchema = z.string()
  .min(1)
  .max(256)
  .describe('The exact id of the channel, as the messenger answers it.');

const channelIDListSchema = z.array(channelIDSchema).max(1000);

export const channelOrderGetInputSchema = z.strictObject({});

export const channelOrderSetInputSchema = z.strictObject({
  pinnedChannels: channelIDListSchema
    .describe('The pinned channels in the order to keep them. It only reorders the pins: a channel named here that is not pinned stays unpinned, and a pinned channel left out keeps its pin at the end.')
    .optional(),
  channelOrder: channelIDListSchema
    .describe('Every channel in the order the requester lists them, pinned ones included at the place they return to when unpinned.')
    .optional(),
});

export const channelPinInputSchema = z.strictObject({
  channelID: channelIDSchema,
});

export const channelOrderResultSchema = z.strictObject({
  pinnedChannels: z.array(z.string()),
  channelOrder: z.array(z.string()),
});

export const channelPinResultSchema = z.strictObject({
  channelID: z.string(),
  isPinned: z.boolean(),
  pinnedChannels: z.array(z.string()),
  channelOrder: z.array(z.string()),
});

const channelOrderEffects = [{ objectType: 'channel_order', effect: ResourceMutationEffect.Updated, effectIdentity: ResourceEffectIdentity.Singleton }];

const channelPinEffects = [{ objectType: 'channel', effect: ResourceMutationEffect.Updated, resultField: 'channelID', effectIdentity: ResourceEffectIdentity.ID }];

function channelOrderTool(
  name: ChannelOrderToolName,
  description: string,
  inputSchema: z.ZodObject,
  result: CapabilityResultDefinition,
  sideEffect: CapabilitySideEffect,
): CapabilityToolDefinition {
  return {
    name,
    namespace: 'channel',
    answeredBy: CapabilityAnsweredBy.Record,
    privacyClass: 'user_preference',
    policyResource: `tool:${name}`,
    description,
    version: '1',
    estimatedLatency: CapabilityEstimatedLatency.Low,
    modelVisibility: CapabilityModelVisibility.Hidden,
    inputSchema,
    result,
    sideEffect,
  };
}

export const channelOrderToolDefinitions: CapabilityToolDefinition[] = [
  channelOrderTool(
    ChannelOrderToolName.Get,
    'Get how the requester orders their own channel list: the channels they pinned, in order, and the order of every channel. It is the requester\'s own and the same on every device they sign in on.',
    channelOrderGetInputSchema,
    { schema: channelOrderResultSchema, effects: [] },
    CapabilitySideEffect.Read,
  ),
  channelOrderTool(
    ChannelOrderToolName.Set,
    'Set the order of the requester\'s own channel list: the order of their pinned channels, the order of every channel, or both. It changes nothing for anybody else.',
    channelOrderSetInputSchema,
    { schema: channelOrderResultSchema, effects: channelOrderEffects },
    CapabilitySideEffect.WorkspaceWrite,
  ),
  channelOrderTool(
    ChannelOrderToolName.Pin,
    'Pin a channel to the top of the requester\'s own channel list, after the channels already pinned. Pinning a pinned channel leaves it where it is, and it changes nothing for anybody else.',
    channelPinInputSchema,
    { schema: channelPinResultSchema, effects: channelPinEffects },
    CapabilitySideEffect.WorkspaceWrite,
  ),
  channelOrderTool(
    ChannelOrderToolName.Unpin,
    'Unpin a channel from the requester\'s own channel list, so it goes back to its place among the other channels.',
    channelPinInputSchema,
    { schema: channelPinResultSchema, effects: channelPinEffects },
    CapabilitySideEffect.WorkspaceWrite,
  ),
];

export type ChannelOrderResult = z.infer<typeof channelOrderResultSchema>;
export type ChannelPinResult = z.infer<typeof channelPinResultSchema>;
