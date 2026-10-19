import { z } from 'zod';

import {
  CapabilityAnsweredBy,
  CapabilityEstimatedLatency,
  CapabilitySideEffect,
  ResourceEffectIdentity,
} from './protocol';
import { ResourceMutationEffect, type CapabilityToolDefinition } from './definition';

export enum ChannelToolName {
  Add = 'channel_add',
  Join = 'channel_join',
  Leave = 'channel_leave',
  Delete = 'channel_delete',
  MemberAdd = 'channel_member_add',
  MemberDelete = 'channel_member_delete',
  OwnerAdd = 'channel_owner_add',
  OwnerUpdate = 'channel_owner_update',
}

export enum ChannelVisibility {
  Open = 'open',
  Private = 'private',
}

const channelHintSchema = z.string()
  .min(1)
  .describe('The exact channel ID, or the channel\'s CURRENT name as the messenger shows it, with or without #. Never a new or intended name.');

const personHintSchema = z.string()
  .min(1)
  .describe('Name, @handle, or email of one company member.');

const personHintsSchema = z.array(personHintSchema).min(1).max(50);

export const channelAddInputSchema = z.strictObject({
  name: z.string().min(1).describe('The new channel\'s name.'),
  visibility: z.enum(ChannelVisibility).describe('open lets anyone in the company find and join it, and takes the assistant in; private holds only the people added.'),
  description: z.string().describe('What the channel is for, shown under its name.').optional(),
  personHints: personHintsSchema.describe('Members to add besides the requester, who joins as its owner.').optional(),
});

export const channelHintInputSchema = z.strictObject({
  channelHint: channelHintSchema,
});

export const channelMembersInputSchema = z.strictObject({
  channelHint: channelHintSchema,
  personHints: personHintsSchema.describe('The members to add.'),
});

export const channelMemberInputSchema = z.strictObject({
  channelHint: channelHintSchema,
  personHint: personHintSchema,
});

const channelResultSchema = z.strictObject({
  channelID: z.string().min(1),
  channelName: z.string(),
  notAddedPersonHints: z.array(z.string()).describe('The people named who were not added; the messenger refused them.').optional(),
});

function channelEffect(effect: ResourceMutationEffect) {
  return [{ objectType: 'channel', effect, resultField: 'channelID', effectIdentity: ResourceEffectIdentity.ID }];
}

function channelTool(
  name: ChannelToolName,
  description: string,
  inputSchema: z.ZodObject,
  effect: ResourceMutationEffect,
  sideEffect: CapabilitySideEffect,
): CapabilityToolDefinition {
  const requiresApproval = sideEffect === CapabilitySideEffect.Destructive || name === ChannelToolName.OwnerUpdate;
  return {
    name,
    namespace: 'channel',
    answeredBy: CapabilityAnsweredBy.Company,
    privacyClass: 'platform_message',
    policyResource: `tool:${name}`,
    description,
    version: '1',
    estimatedLatency: CapabilityEstimatedLatency.Medium,
    inputSchema,
    inputIntentSchema: inputSchema.partial(),
    result: { schema: channelResultSchema, effects: channelEffect(effect) },
    sideEffect,
    requiresApproval,
  };
}

const asTheRequester = 'It is done as the requester, exactly as their messenger screen does it, and refused for the same reasons.';
const ownersOnly = 'Only an owner of the channel may do this.';

export const channelToolDefinitions: CapabilityToolDefinition[] = [
  channelTool(ChannelToolName.Add,
    `Create a group channel in the company messenger with the requester as its owner, and add the members named. ${asTheRequester}`,
    channelAddInputSchema, ResourceMutationEffect.Created, CapabilitySideEffect.ExternalWrite),
  channelTool(ChannelToolName.Join,
    `Join an open channel the requester is not in yet. ${asTheRequester}`,
    channelHintInputSchema, ResourceMutationEffect.Updated, CapabilitySideEffect.ExternalWrite),
  channelTool(ChannelToolName.Leave,
    `Leave a channel the requester is in. The only owner cannot leave; hand the channel over first with channel_owner_update. ${asTheRequester}`,
    channelHintInputSchema, ResourceMutationEffect.Updated, CapabilitySideEffect.ExternalWrite),
  channelTool(ChannelToolName.Delete,
    `Permanently delete a channel for everyone in it. ${ownersOnly} ${asTheRequester} Requires approval; this action is irreversible.`,
    channelHintInputSchema, ResourceMutationEffect.Deleted, CapabilitySideEffect.Destructive),
  channelTool(ChannelToolName.MemberAdd,
    `Add company members to a channel the requester is in. ${asTheRequester}`,
    channelMembersInputSchema, ResourceMutationEffect.Updated, CapabilitySideEffect.ExternalWrite),
  channelTool(ChannelToolName.MemberDelete,
    `Remove one member from a channel. ${ownersOnly} An owner cannot be removed. ${asTheRequester}`,
    channelMemberInputSchema, ResourceMutationEffect.Updated, CapabilitySideEffect.ExternalWrite),
  channelTool(ChannelToolName.OwnerAdd,
    `Make a member of a channel a co-owner beside the requester. ${ownersOnly} ${asTheRequester}`,
    channelMemberInputSchema, ResourceMutationEffect.Updated, CapabilitySideEffect.ExternalWrite),
  channelTool(ChannelToolName.OwnerUpdate,
    `Hand a channel over: the person named becomes its owner and the requester stays as a plain member, which only another owner can undo. ${ownersOnly} ${asTheRequester} Requires approval.`,
    channelMemberInputSchema, ResourceMutationEffect.Updated, CapabilitySideEffect.ExternalWrite),
];

export type ChannelToolResult = z.infer<typeof channelResultSchema>;
