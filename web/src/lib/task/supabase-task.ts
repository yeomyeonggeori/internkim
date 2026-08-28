import { supabase } from '$lib/supabase';
import { announceTaskMoved } from '$lib/task/announce-task';
import { taskDefinitionsOf, taskVocabularyOfDefinitions, vocabularyOf } from '$lib/task/task-vocabulary';
import { heldTasks, holdTasks, mergeChangedTasks, newestStamp } from '$lib/task/task-cache';
import {
	centralTaskStatusOptions,
	centralTaskFromRow,
	centralTaskSelection,
	centralTaskWriteFields,
	centralStatusFromWord,
	type CentralTaskRow
} from '$lib/task/central-task';
import {
	currentScoresOf,
	memberScoreDetails,
	memberTaskTallies,
	startOfISOWeek,
	totalScoreOf,
	type MemberTaskTally
} from '$lib/task/task-scores';
import type {
	TaskDefinitions,
	TaskMember,
	TaskMemberScoreDetail,
	TaskMetrics,
	TaskState,
	Task,
	TaskWeek,
	TaskWeeklySummary
} from '../../routes/task/task-types';

export const taskStatusOptions = centralTaskStatusOptions;

type MemberRow = { id: string; name: string | null; email: string | null; is_admin: boolean; user_id: string | null; joined_at: string | null };
type TaskRow = CentralTaskRow;

// The API caps an unbounded select and says nothing about having done it, so a
// company past the cap would quietly lose tasks off its board. Rows are read a
// page at a time, ordered by id, because ordering by anything that repeats drops
// rows at a page boundary.
const rowsPerPage = 500;

async function everyRow<Row>(
	page: (from: number, to: number) => PromiseLike<{ data: Row[] | null; error: { message: string } | null }>
): Promise<Row[]> {
	const rows: Row[] = [];
	for (let from = 0; ; from += rowsPerPage) {
		const read = await page(from, from + rowsPerPage - 1);
		if (read.error) throw new Error(read.error.message);
		const found = read.data ?? [];
		rows.push(...found);
		if (found.length < rowsPerPage) return rows;
	}
}

async function readTasks(): Promise<TaskRow[]> {
	const client = supabase();
	const held = heldTasks<TaskRow>();

	if (!held) {
		const everything = await everyRow<TaskRow>((from, to) => client.from('task').select(centralTaskSelection).order('id').range(from, to));
		holdTasks({ tasks: everything, fetchedAt: newestStamp(everything) });
		return sortedByEnd(everything);
	}

	const changed = await everyRow<TaskRow>((from, to) =>
		client.from('task').select(centralTaskSelection).gte('updated_at', held.fetchedAt).order('id').range(from, to)
	);
	const live = await everyRow<{ id: string }>((from, to) => client.from('task').select('id').order('id').range(from, to));

	const merged = mergeChangedTasks(held.tasks, changed, new Set(live.map((row) => row.id)));
	holdTasks({ tasks: merged, fetchedAt: newestStamp(merged) });
	return sortedByEnd(merged);
}

function sortedByEnd(tasks: TaskRow[]): TaskRow[] {
	return [...tasks].sort((left, right) => (right.ends_at ?? '').localeCompare(left.ends_at ?? ''));
}

type MemberStanding = {
	key: string;
	scoreDetails: Record<string, TaskMemberScoreDetail>;
	tallies: Record<string, MemberTaskTally>;
};

let heldStanding: MemberStanding | null = null;

function standingOf(tasks: Task[], memberIDs: string[], rows: TaskRow[]): MemberStanding {
	const weekStart = startOfISOWeek(new Date());
	const key = standingKey(rows, memberIDs, weekStart);
	if (heldStanding?.key === key) return heldStanding;
	heldStanding = {
		key,
		scoreDetails: memberScoreDetails(tasks, memberIDs, weekStart),
		tallies: memberTaskTallies(tasks, memberIDs)
	};
	return heldStanding;
}

function standingKey(rows: TaskRow[], memberIDs: string[], weekStart: Date): string {
	return [rows.length, newestStamp(rows), dayStringOf(weekStart), memberIDs.join(',')].join('|');
}

function standingMetricsOf(standing: MemberStanding): Partial<TaskMetrics> {
	const memberScores = currentScoresOf(standing.scoreDetails);
	const memberDistances = Object.fromEntries(
		Object.entries(standing.tallies).map(([memberID, tally]) => [memberID, tally.distance])
	);
	return {
		memberScores,
		memberScoreDetails: standing.scoreDetails,
		memberDistances,
		totalScore: totalScoreOf(memberScores),
		totalDistance: Object.values(memberDistances).reduce((total, distance) => total + distance, 0)
	};
}

function dayStringOf(date: Date): string {
	return date.toISOString().slice(0, 10);
}

export async function supabaseTaskState(): Promise<TaskState> {
	const client = supabase();
	const { data: auth } = await client.auth.getSession();
	const accountID = auth.session?.user.id ?? '';

	const company = await client
		.from('company')
		.select('task_vocabulary, timezone')
		.limit(1)
		.single<{ task_vocabulary: unknown; timezone: string | null }>();
	if (company.error) throw new Error(company.error.message);

	const members = await client
		.from('member')
		.select('id, name, email, is_admin, user_id, joined_at')
		.neq('status', 'withdrawn')
		.returns<MemberRow[]>();
	if (members.error) throw new Error(members.error.message);

	const taskRows = await readTasks();

	const nameByID = new Map(members.data.map((member) => [member.id, displayName(member)]));
	const timeZone = company.data.timezone || 'UTC';
	const tasks = taskRows.map((task) => taskOf(task, nameByID, timeZone));
	const memberIDs = members.data.map((member) => member.id);
	const standing = standingOf(tasks, memberIDs, taskRows);
	const me = members.data.find((member) => member.user_id === accountID);

	return {
		currentWeek: weekOf(new Date()),
		members: members.data.map((member) => memberOf(member, standing.tallies[member.id])),
		tasks: tasks,
		metrics: { ...metricsOf(tasks), ...standingMetricsOf(standing) },
		definitions: taskDefinitionsOf(vocabularyOf(company.data.task_vocabulary)),
		statusOptions: taskStatusOptions,
		currentUserEmail: me?.email ?? '',
		currentUserName: me ? displayName(me) : '',
		isAdmin: me?.is_admin ?? false,
		source: 'supabase'
	};
}

export async function supabaseTaskWeeklySummary(week: string): Promise<TaskWeeklySummary> {
	const state = await supabaseTaskState();
	const shown = week ? weekOfCode(week) : weekOf(new Date());
	const weeklyTasks = state.tasks.filter((task) => task.weekCode === shown.code);
	return {
		week: shown,
		currentWeek: state.currentWeek,
		weeklyTasks,
		metrics: metricsOf(weeklyTasks),
		source: 'supabase'
	};
}

// The board writes a day. An event was given hours, and rewriting those from a
// board edit would move a meeting nobody asked to move, so an event keeps its own.
export function savedTaskFields(task: Task, operation: 'insert' | 'update' = 'update'): Record<string, unknown> {
	return centralTaskWriteFields(task, operation);
}

export type SupabaseTaskRPCArguments = {
	target_task_id: string | null;
	target_title: string;
	target_status: string;
	target_note: string | null;
	target_business: string | null;
	target_type: string | null;
	target_size: string | null;
	target_starts_at: string | null;
	target_ends_at: string | null;
	target_write_dates: boolean;
	target_is_event: boolean;
	target_participant_ids: string[];
	target_parent_task_id: string | null;
};

export function supabaseTaskRPCArguments(task: Task): SupabaseTaskRPCArguments {
	const operation = task.id ? 'update' : 'insert';
	const fields = savedTaskFields(task, operation);
	return {
		target_task_id: task.id || null,
		target_title: requiredStringField(fields, 'title'),
		target_status: requiredStringField(fields, 'status'),
		target_note: nullableStringField(fields, 'note'),
		target_business: nullableStringField(fields, 'business'),
		target_type: nullableStringField(fields, 'type'),
		target_size: nullableStringField(fields, 'size'),
		target_starts_at: nullableStringField(fields, 'starts_at'),
		target_ends_at: nullableStringField(fields, 'ends_at'),
		target_write_dates: !task.isEvent,
		target_is_event: task.isEvent === true,
		target_participant_ids: task.participantIDs,
		target_parent_task_id: operation === 'insert' ? nullableStringField(fields, 'parent_task_id') : null
	};
}

function requiredStringField(fields: Record<string, unknown>, field: string): string {
	const value = fields[field];
	if (typeof value !== 'string') throw new Error(`task ${field} must be a string`);
	return value;
}

function nullableStringField(fields: Record<string, unknown>, field: string): string | null {
	const value = fields[field];
	if (value === undefined || value === null) return null;
	if (typeof value !== 'string') throw new Error(`task ${field} must be a string or null`);
	return value;
}

export async function saveSupabaseTask(task: Task): Promise<void> {
	const saved = await supabase().rpc('task_save', supabaseTaskRPCArguments(task));
	if (saved.error) throw new Error(saved.error.message);
}

export async function saveSupabaseTaskVocabulary(
	definitions: TaskDefinitions,
	messages: { failure: string; inUse: string }
): Promise<void> {
	const saved = await supabase().rpc('task_vocabulary_save', {
		target_vocabulary: taskVocabularyOfDefinitions(definitions)
	});
	if (!saved.error) return;
	if (saved.error.code === '2BP01') throw new Error(messages.inUse);
	throw new Error(messages.failure);
}

export async function deleteSupabaseTask(taskID: string): Promise<void> {
	const { error } = await supabase()
		.from('task')
		.delete()
		.eq('id', taskID)
		.select('id')
		.single<{ id: string }>();
	if (error) throw new Error(error.message);
}

export async function moveSupabaseTask(taskID: string, status: string): Promise<void> {
	const { error } = await supabase()
		.from('task')
		.update({ status: centralStatusFromWord(status) })
		.eq('id', taskID)
		.select('id')
		.single<{ id: string }>();
	if (error) throw new Error(error.message);
	void announceTaskMoved(taskID);
}

function displayName(member: MemberRow): string {
	return member.name || (member.email ?? '').split('@')[0];
}

function memberOf(member: MemberRow, tally: MemberTaskTally | undefined): TaskMember {
	return {
		id: member.id,
		name: displayName(member),
		email: member.email ?? '',
		hireDate: member.joined_at ? member.joined_at.slice(0, 10) : undefined,
		role: member.is_admin ? 'admin' : 'member',
		mattermostStatus: '',
		distance: tally?.distance ?? 0,
		activeTaskCount: tally?.activeTaskCount ?? 0,
		completeTaskCount: tally?.completeTaskCount ?? 0
	};
}

function taskOf(task: TaskRow, nameByID: Map<string, string>, timeZone: string): Task {
	return centralTaskFromRow(
		task,
		nameByID,
		(instant) => dayOf(instant, timeZone),
		(day) => weekOf(new Date(`${day}T00:00:00Z`)).code
	);
}

function metricsOf(tasks: Task[]): TaskMetrics {
	const statusCounts: Record<string, number> = {};
	const businessCounts: Record<string, number> = {};
	const typeCounts: Record<string, number> = {};
	for (const task of tasks) {
		statusCounts[task.status] = (statusCounts[task.status] ?? 0) + 1;
		if (task.business) businessCounts[task.business] = (businessCounts[task.business] ?? 0) + 1;
		if (task.type) typeCounts[task.type] = (typeCounts[task.type] ?? 0) + 1;
	}
	return {
		totalTasks: tasks.length,
		completedTasks: statusCounts['completed'] ?? 0,
		requestedTasks: statusCounts['requested'] ?? 0,
		pausedTasks: statusCounts['paused'] ?? 0,
		stoppedTasks: statusCounts['stopped'] ?? 0,
		statusCounts,
		businessCounts,
		typeCounts
	};
}

// A stored instant is a moment, and which day it falls on depends on where you
// stand. Slicing the text reads it in UTC, which puts a Seoul midnight on the
// day before. The company's own zone decides, so everyone on the board sees the
// same day for the same row.
// Postgres prints an hours-only offset, `+00`, which no Date parser accepts.
function isoInstantOf(instant: string): string {
	return instant.replace(' ', 'T').replace(/([+-]\d{2})$/, '$1:00');
}

export function dayOf(instant: string | null, timeZone: string): string | undefined {
	if (!instant) return undefined;
	const moment = new Date(isoInstantOf(instant));
	if (Number.isNaN(moment.getTime())) return undefined;
	return new Intl.DateTimeFormat('en-CA', { timeZone, year: 'numeric', month: '2-digit', day: '2-digit' }).format(moment);
}

function weekOf(instant: Date): TaskWeek {
	return weekFromMonday(startOfISOWeek(instant), true);
}

function weekOfCode(code: string): TaskWeek {
	const monday = new Date(`${code}T00:00:00Z`);
	if (Number.isNaN(monday.getTime())) return weekOf(new Date());
	return weekFromMonday(monday, code === weekOf(new Date()).code);
}

function weekFromMonday(monday: Date, isCurrent: boolean): TaskWeek {
	const sunday = new Date(monday);
	sunday.setUTCDate(sunday.getUTCDate() + 6);
	return {
		code: monday.toISOString().slice(0, 10),
		startISO: monday.toISOString().slice(0, 10),
		endISO: sunday.toISOString().slice(0, 10),
		previous: shiftedWeek(monday, -7),
		next: shiftedWeek(monday, 7),
		isCurrent
	};
}

function shiftedWeek(monday: Date, days: number): string {
	const moved = new Date(monday);
	moved.setUTCDate(moved.getUTCDate() + days);
	return moved.toISOString().slice(0, 10);
}
