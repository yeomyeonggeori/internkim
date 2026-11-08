import { z } from 'zod';
import type { Task } from '../../routes/task/task-types';

const count = z.number().int().nonnegative();
const namedColorSchema = z.object({ name: z.string(), color: z.string().optional() });

const sharedTaskSchema = z.object({
	id: z.string(),
	parentID: z.string().optional(),
	title: z.string(),
	status: z.string(),
	business: z.string().optional(),
	type: z.string().optional(),
	size: z.string().optional(),
	startDate: z.string().optional(),
	endDate: z.string().optional(),
	createdAt: z.string().optional(),
	people: z.array(z.string())
});

export const sharedTaskBoardSchema = z.object({
	company: z.object({ name: z.string() }),
	businesses: z.array(namedColorSchema),
	types: z.array(namedColorSchema),
	counts: z.record(z.string(), count),
	tasks: z.array(sharedTaskSchema),
	people: z.record(z.string(), z.object({ name: z.string() }))
});

export type SharedTaskBoard = z.infer<typeof sharedTaskBoardSchema>;

export const acceptedTaskInvitationSchema = z.object({ companyID: z.guid() });

export function sharedTasks(board: SharedTaskBoard): Task[] {
	return board.tasks.map((task) => {
		const names = task.people.map((person) => board.people[person]?.name ?? '');
		return {
			id: task.id,
			parentTaskID: task.parentID,
			ownerID: task.people[0] ?? '',
			ownerName: names[0] ?? '',
			participantIDs: task.people,
			participantNames: names,
			business: task.business ?? null,
			type: task.type ?? null,
			content: task.title,
			size: task.size ?? '',
			status: task.status,
			startDate: task.startDate,
			endDate: task.endDate,
			createdAt: task.createdAt,
			weekCode: ''
		};
	});
}

export function sharedTaskTotal(board: SharedTaskBoard): number {
	return Object.values(board.counts).reduce((sum, total) => sum + total, 0);
}
