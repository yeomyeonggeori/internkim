import type { SupabaseClient } from '@supabase/supabase-js';
import { taskWeekDateFromISO, taskWeekOfDate } from '$lib/task/task-week-code';
import type { TaskBoardOptions, TaskBoardWeekPosition } from '../../../../routes/task/task-board-model';
import type { TaskChildProgress } from '../../../../routes/task/task-relationships';
import { dayShifted, instantOfDay } from './days';
import { workColumn, type TaskRow } from './tasks';

export type TaskCardRow = Pick<TaskRow,
	'id' | 'parent_task_id' | 'title' | 'status' | 'business' | 'type' | 'size' |
	'starts_at' | 'ends_at' | 'task_last_day' | 'created_at' | 'updated_at' | 'requester_id' | 'task_participant' | 'task_deletable' | 'task_updatable'
> & Partial<Pick<TaskRow, 'organization_id' | 'opportunity_id'>>;

export const cardColumns = 'id, parent_task_id, title, status, business, type, size, starts_at, ends_at, task_last_day, created_at, updated_at, requester_id, task_participant (member_id), task_deletable, task_updatable';
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

export function lastDayBetween(timeZone: string, firstDay: string, lastDay: string): string {
	return `${lastDayOnOrAfter(timeZone, firstDay)},${lastDayOnOrBefore(timeZone, lastDay)}`;
}

function lastDayOnOrAfter(timeZone: string, day: string): string {
	const start = instantOfDay(timeZone, day);
	return `or(and(is_whole_day.is.true,ends_at.gt.${start}),and(is_whole_day.is.false,ends_at.gte.${start}))`;
}

function lastDayOnOrBefore(timeZone: string, day: string): string {
	const end = instantOfDay(timeZone, dayShifted(day, 1));
	return `or(and(is_whole_day.is.true,ends_at.lte.${end}),and(is_whole_day.is.false,ends_at.lt.${end}))`;
}

export function taskBoardDatePredicate(window: TaskBoardWindow): string {
	const weekEnd = dayShifted(window.week, 6);
	const endsThisWeek = lastDayBetween(window.timeZone, window.week, weekEnd);
	const clauses = [
		'status.in.(requested,paused)',
		`and(status.eq.completed,${endsThisWeek})`
	];
	if (window.position === 'current') {
		clauses.push('status.eq.in_progress');
		clauses.push(`and(status.eq.planned,or(${lastDayOnOrBefore(window.timeZone, weekEnd)},and(ends_at.is.null,or(starts_at.lt.${window.until},starts_at.is.null))))`);
	}
	if (window.position === 'future') {
		clauses.push(`and(status.eq.planned,or(and(${endsThisWeek}),and(ends_at.is.null,starts_at.gte.${window.from},starts_at.lt.${window.until})))`);
	}
	return clauses.join(',');
}

export async function taskChildProgressForBoard(caller: SupabaseClient, taskIDs: string[]): Promise<Record<string, TaskChildProgress>> {
	const progress: Record<string, TaskChildProgress> = {};
	const parents = [...new Set(taskIDs)];
	for (let offset = 0; offset < parents.length; offset += parentIDsPerQuery) {
		const parentIDs = parents.slice(offset, offset + parentIDsPerQuery);
		for (let from = 0; ; from += rowsPerPage) {
			const { data, error } = await caller.from('task').select('id, parent_task_id, status')
				.eq(workColumn, true).in('parent_task_id', parentIDs).neq('status', 'stopped')
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
