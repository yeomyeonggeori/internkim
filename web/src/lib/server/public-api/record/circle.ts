import { z } from 'zod';
import { circleListResultSchema } from '$lib/data-room/schemas';
import { circleMemberSetInputSchema, circleSetInputSchema } from '../catalog/circle';
import type { RecordContext } from './company';
import { companyRows, dataRoomCall } from './data-room';

const circleRowSchema = z.object({ id: z.string(), name: z.string(), name_ko: z.string() });
const readRowSchema = z.object({ circle_id: z.string(), category_code: z.string() });
const memberRowSchema = z.object({ circle_id: z.string(), member_id: z.string() });

export async function companyCircleList(context: RecordContext) {
	const [circles, reads, members] = await Promise.all([
		companyRows(context.caller, context.companyID, 'circle'),
		companyRows(context.caller, context.companyID, 'circle_category'),
		companyRows(context.caller, context.companyID, 'circle_member')
	]);
	const categories = z.array(readRowSchema).parse(reads);
	const memberships = z.array(memberRowSchema).parse(members);
	return circleListResultSchema.parse({
		circles: z.array(circleRowSchema).parse(circles).map((circle) => ({
			id: circle.id, name: circle.name, nameKO: circle.name_ko,
			readableCategories: categories.filter((row) => row.circle_id === circle.id).map((row) => row.category_code),
			memberIDs: memberships.filter((row) => row.circle_id === circle.id).map((row) => row.member_id)
		}))
	});
}

export async function companyCircleSet(context: RecordContext, value: unknown) {
	const circle = circleSetInputSchema.parse(value);
	await dataRoomCall(context.caller, 'circle_set', {
		target_company: context.companyID, circle_id: circle.id, circle_name: circle.name,
		circle_name_ko: circle.nameKO, readable_categories: circle.readableCategories
	});
	return { saved: true };
}

export async function companyCircleMemberSet(context: RecordContext, value: unknown) {
	const assignment = circleMemberSetInputSchema.parse(value);
	await dataRoomCall(context.caller, 'member_circles_set', {
		target_company: context.companyID, target_member: assignment.memberID, circle_ids: assignment.circleIDs
	});
	return { saved: true };
}
