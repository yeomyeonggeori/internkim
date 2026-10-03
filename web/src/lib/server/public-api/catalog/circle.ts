import { z } from 'zod';
import { circleSchema } from '$lib/data-room/model';
import { circleListResultSchema } from '$lib/data-room/schemas';
import { companyRecordTool, recordSavedSchema } from './data-room';
import type { CapabilityToolDefinition } from './definition';

export const circleSetInputSchema = circleSchema;
export const circleMemberSetInputSchema = z.strictObject({ memberID: z.string().uuid(), circleIDs: z.array(z.string()) });

const circleTool = companyRecordTool('circle', 'circle', 'Company circles');

export const circleToolDefinitions: CapabilityToolDefinition[] = [
	circleTool('circle_list',
		'Read every circle with the members who belong to it and the data room categories it reads. A circle is also a shared workspace folder and a shared memory on the company computer. A parent category grants all its children.',
		z.strictObject({}), circleListResultSchema),
	circleTool('circle_update',
		'Create or edit a circle. id is its lowercase identifier, such as leadership, and never changes; name and nameKO are what people read. readableCategories names the data room categories its members read, and a parent grants all descendants. Members change access immediately, so read circle_list and confirm who is affected first. A circle confers no administrative or editing rights.',
		circleSetInputSchema, recordSavedSchema, true),
	circleTool('circle_member_update',
		'Set the circles an active member belongs to, replacing them atomically. Only administrators place people in circles. A member gains each circle\'s workspace folder, memory and data room categories.',
		circleMemberSetInputSchema, recordSavedSchema, true)
];
