import { z } from 'zod';
import { circleIDSchema } from '$lib/data-room/model';
import { circleListResultSchema } from '$lib/data-room/schemas';
import { companyRecordTool, recordSavedSchema } from './data-room';
import { ResourceMutationEffect, type CapabilityToolDefinition } from './definition';
import { CapabilitySideEffect, ResourceEffectIdentity } from './protocol';

export const circleSetInputSchema = z.strictObject({
	id: circleIDSchema.describe('The lowercase English name the company host gives its folder and group, such as sales-team; it never changes'),
	name: z.string().trim().min(1).optional().describe('What people call a circle the company made. Required for one; a default circle takes none, because the product names it'),
	readableCategories: z.array(z.string())
});
export const circleMemberSetInputSchema = z.strictObject({ memberID: z.string().uuid(), circleIDs: z.array(z.string()) });
export const circleChannelAddInputSchema = z.strictObject({
	circleID: z.string().describe('The exact id of the circle, as circle_list shows it'),
	name: z.string().trim().min(1).describe("What the channel is called on the messenger, usually the circle's own name")
});
export const circleChannelDeleteInputSchema = z.strictObject({
	circleID: z.string().describe('The exact id of the circle, as circle_list shows it'),
	name: z.string().describe("The channel's exact current name, as circle_list shows it in channelName. Any other value is refused.")
});

const circleTool = companyRecordTool('circle', 'circle', 'Company circles');

export const circleToolDefinitions: CapabilityToolDefinition[] = [
	circleTool('circle_list',
		'Read every circle with the members who belong to it and the data room categories it reads. A circle is also a shared workspace folder and a shared memory on the company computer. A parent category grants all its children.',
		z.strictObject({}), circleListResultSchema),
	circleTool('circle_update',
		'Create or edit a circle. id is its lowercase English name, such as sales-team, and never changes; name is what people read and is given only to a circle the company made, since the product names its default circles in every language. readableCategories names the data room categories its members read, and a parent grants all descendants. Members change access immediately, so read circle_list and confirm who is affected first. A circle confers no administrative or editing rights.',
		circleSetInputSchema, recordSavedSchema, true),
	circleTool('circle_member_update',
		'Set the circles an active member belongs to, replacing them atomically. Only administrators place people in circles. A member gains each circle\'s workspace folder, memory and data room categories, and joins its channel when it has one.',
		circleMemberSetInputSchema, recordSavedSchema, true),
	circleTool('circle_channel_add',
		"Add a private messenger channel of the circle's own. Its members are exactly the circle's, kept so as people join and leave the circle, and nobody adds or removes anyone by hand. A circle has at most one channel. Only administrators add one.",
		circleChannelAddInputSchema, recordSavedSchema, true),
	{
		...circleTool('circle_channel_delete',
			"Delete the circle's messenger channel, so nobody reads or writes in it again. Takes the channel's exact current name from circle_list as confirmation and refuses any other. Only administrators delete one, and the circle itself stays.",
			circleChannelDeleteInputSchema, recordSavedSchema, true),
		result: {
			schema: recordSavedSchema,
			effects: [{ objectType: 'circle', effect: ResourceMutationEffect.Deleted, effectIdentity: ResourceEffectIdentity.Singleton }]
		},
		sideEffect: CapabilitySideEffect.Destructive
	}
];
