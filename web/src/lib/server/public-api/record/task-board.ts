import type { SupabaseClient } from '@supabase/supabase-js';
import { taskWeekDateFromISO, taskWeekOfDate } from '$lib/task/task-week-code';
import { matchesTaskBoardWeek, type TaskBoardOptions, type TaskBoardWeekPosition } from '../../../../routes/task/task-board-model';
import type { TaskChildProgress } from '../../../../routes/task/task-relationships';
import { dayOfInstant, dayShifted, instantOfDay } from './days';
import type { TaskRow } from './tasks';

export type TaskCardRow = Pick<TaskRow,
	'id' | 'parent_task_id' | 'title' | 'status' | 'business' | 'type' | 'size' |
	'starts_at' | 'ends_at' | 'created_at' | 'updated_at' | 'requester_id' | 'task_participant'
> & Partial<Pick<TaskRow, 'organization_id' | 'opportunity_id'>>;

const cardColumns = 'id, parent_task_id, title, status, business, type, size, starts_at, ends_at, created_at, updated_at, requester_id, task_participant (member_id)';
const rowsPerPage = 500;
const parentIDsPerQuery = 100;

export type TaskBoardWindow = {
	week: string;
	timeZone: string;
	from: string;
	until: string;
	position: TaskBoardWeekPosition;
	options: TaskBoardOptions;
};

export function taskBoardWindow(week: string, timeZone: string, now: Date): TaskBoardWindow {
	const monday = taskWeekDateFromISO(week);
	if (!monday || monday.getUTCDay() !== 1) throw new Error('boardWeek must name a Monday as YYYY-MM-DD');
	const current = taskWeekOfDate(now).startISO;
	const position = week < current ? 'past' : week > current ? 'future' : 'current';
	return {
		week, timeZone, position,
		from: instantOfDay(timeZone, week),
		until: instantOfDay(timeZone, dayShifted(week, 7)),
		options: { weekStartISO: week, weekEndISO: dayShifted(week, 6), weekPosition: position }
	};
}

export function taskBoardDatePredicate(window: TaskBoardWindow): string {
	const clauses = [
		'status.in.(requested,paused)',
		`and(status.eq.completed,ends_at.gte.${window.from},ends_at.lt.${window.until})`
	];
	if (window.position === 'current') {
		clauses.push('status.eq.in_progress');
		clauses.push(`and(status.eq.planned,or(ends_at.lt.${window.until},and(ends_at.is.null,or(starts_at.lt.${window.until},starts_at.is.null))))`);
	}
	if (window.position === 'future') {
		clauses.push(`and(status.eq.planned,or(and(ends_at.gte.${window.from},ends_at.lt.${window.until}),and(ends_at.is.null,starts_at.gte.${window.from},starts_at.lt.${window.until})))`);
	}
	return clauses.join(',');
}

export function rowMatchesTaskBoard(row: TaskCardRow, window: TaskBoardWindow): boolean {
	return matchesTaskBoardWeek({
		status: row.status,
		startDate: dayOfInstant(window.timeZone, row.starts_at),
		endDate: dayOfInstant(window.timeZone, row.ends_at)
	}, window.options);
}

export async function tasksForBoard(
	caller: SupabaseClient,
	window: TaskBoardWindow,
	links: { organizationID?: string; opportunityID?: string } = {}
): Promise<TaskCardRow[]> {
	const rows: TaskCardRow[] = [];
	for (let from = 0; ; from += rowsPerPage) {
		let query = caller.from('task').select(cardColumns).eq('is_event', false)
			.or(taskBoardDatePredicate(window));
		if (links.organizationID) query = query.eq('organization_id', links.organizationID);
		if (links.opportunityID) query = query.eq('opportunity_id', links.opportunityID);
		const { data, error } = await query.order('id').range(from, from + rowsPerPage - 1).returns<TaskCardRow[]>();
		if (error) throw new Error(error.message);
		const page = data ?? [];
		rows.push(...page.filter(row => rowMatchesTaskBoard(row, window)));
		if (page.length < rowsPerPage) return rows.sort((left, right) => right.updated_at.localeCompare(left.updated_at));
	}
}

export async function taskChildProgressForBoard(caller: SupabaseClient, taskIDs: string[]): Promise<Record<string, TaskChildProgress>> {
	const progress: Record<string, TaskChildProgress> = {};
	const parents = [...new Set(taskIDs)];
	for (let offset = 0; offset < parents.length; offset += parentIDsPerQuery) {
		const parentIDs = parents.slice(offset, offset + parentIDsPerQuery);
		for (let from = 0; ; from += rowsPerPage) {
			const { data, error } = await caller.from('task').select('id, parent_task_id, status')
				.eq('is_event', false).in('parent_task_id', parentIDs).neq('status', 'stopped')
				.order('id').range(from, from + rowsPerPage - 1)
				.returns<{ id: string; parent_task_id: string; status: string }[]>();
			if (error) throw new Error(error.message);
			const children = data ?? [];
			for (const child of children) {
				const tally = progress[child.parent_task_id] ?? { completed: 0, total: 0, percent: 0 };
				tally.total += 1;
				if (child.status === 'completed') tally.completed += 1;
				tally.percent = Math.round(100 * tally.completed / tally.total);
				progress[child.parent_task_id] = tally;
			}
			if (children.length < rowsPerPage) break;
		}
	}
	return progress;
}
