import { supabase } from '$lib/supabase';
import { flowDefinitionsOf, vocabularyOf } from '$lib/flow/task-vocabulary';
import { heldTasks, holdTasks, mergeChangedTasks, newestStamp } from '$lib/flow/flow-task-cache';
import {
	currentScoresOf,
	memberScoreDetails,
	memberTaskTallies,
	startOfISOWeek,
	totalScoreOf,
	type MemberTaskTally
} from '$lib/flow/flow-scores';
import type {
	FlowMember,
	FlowMemberScoreDetail,
	FlowMetrics,
	FlowState,
	FlowTask,
	FlowWeek,
	FlowWeeklySummary
} from '../../routes/flow/flow-types';

type TaskStatus = 'todo' | 'in_progress' | 'done' | 'cancelled' | 'paused';

const statusWords: Record<TaskStatus, string> = {
	todo: '예정',
	in_progress: '진행',
	done: '완료',
	paused: '일시정지',
	cancelled: '중단'
};

const statusOf = Object.fromEntries(
	Object.entries(statusWords).map(([status, word]) => [word, status])
) as Record<string, TaskStatus>;

export const flowStatusOptions = Object.values(statusWords);

type MemberRow = { id: string; name: string | null; email: string | null; is_admin: boolean; user_id: string | null; joined_at: string | null };
type TaskRow = {
	id: string;
	title: string;
	status: TaskStatus;
	note: string | null;
	business: string | null;
	type: string | null;
	size: string | null;
	starts_at: string | null;
	ends_at: string | null;
	due_at: string | null;
	updated_at: string;
	task_participant: { member_id: string }[];
};

const taskColumns =
	'id, title, status, note, business, type, size, starts_at, ends_at, due_at, updated_at, task_participant (member_id)';

async function readTasks(): Promise<TaskRow[]> {
	const client = supabase();
	const held = heldTasks<TaskRow>();

	if (!held) {
		const everything = await client.from('task').select(taskColumns).eq('is_event', false).returns<TaskRow[]>();
		if (everything.error) throw new Error(everything.error.message);
		holdTasks({ tasks: everything.data, fetchedAt: newestStamp(everything.data) });
		return sortedByEnd(everything.data);
	}

	const changed = await client
		.from('task')
		.select(taskColumns)
		.eq('is_event', false)
		.gte('updated_at', held.fetchedAt)
		.returns<TaskRow[]>();
	if (changed.error) throw new Error(changed.error.message);

	const live = await client.from('task').select('id').eq('is_event', false).returns<{ id: string }[]>();
	if (live.error) throw new Error(live.error.message);

	const merged = mergeChangedTasks(held.tasks, changed.data, new Set(live.data.map((row) => row.id)));
	holdTasks({ tasks: merged, fetchedAt: newestStamp(merged) });
	return sortedByEnd(merged);
}

function sortedByEnd(tasks: TaskRow[]): TaskRow[] {
	return [...tasks].sort((left, right) => (right.ends_at ?? '').localeCompare(left.ends_at ?? ''));
}

type MemberStanding = {
	key: string;
	scoreDetails: Record<string, FlowMemberScoreDetail>;
	tallies: Record<string, MemberTaskTally>;
};

let heldStanding: MemberStanding | null = null;

function standingOf(tasks: FlowTask[], memberIDs: string[], rows: TaskRow[]): MemberStanding {
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

function standingMetricsOf(standing: MemberStanding): Partial<FlowMetrics> {
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

export async function supabaseFlowState(): Promise<FlowState> {
	const client = supabase();
	const { data: auth } = await client.auth.getSession();
	const accountID = auth.session?.user.id ?? '';

	const company = await client.from('company').select('task_vocabulary').limit(1).single<{ task_vocabulary: unknown }>();
	if (company.error) throw new Error(company.error.message);

	const members = await client
		.from('member')
		.select('id, name, email, is_admin, user_id, joined_at')
		.neq('status', 'withdrawn')
		.returns<MemberRow[]>();
	if (members.error) throw new Error(members.error.message);

	const tasks = await readTasks();

	const nameByID = new Map(members.data.map((member) => [member.id, displayName(member)]));
	const flowTasks = tasks.map((task) => taskOf(task, nameByID));
	const memberIDs = members.data.map((member) => member.id);
	const standing = standingOf(flowTasks, memberIDs, tasks);
	const me = members.data.find((member) => member.user_id === accountID);

	return {
		currentWeek: weekOf(new Date()),
		members: members.data.map((member) => memberOf(member, standing.tallies[member.id])),
		tasks: flowTasks,
		metrics: { ...metricsOf(flowTasks), ...standingMetricsOf(standing) },
		definitions: flowDefinitionsOf(vocabularyOf(company.data.task_vocabulary)),
		statusOptions: flowStatusOptions,
		currentUserEmail: me?.email ?? '',
		currentUserName: me ? displayName(me) : '',
		isAdmin: me?.is_admin ?? false,
		source: 'supabase'
	};
}

export async function supabaseFlowWeeklySummary(week: string): Promise<FlowWeeklySummary> {
	const state = await supabaseFlowState();
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

export async function saveSupabaseFlowTask(task: FlowTask): Promise<void> {
	const client = supabase();
	const fields = {
		title: task.content || task.goal || '(제목 없음)',
		status: statusOf[task.status] ?? 'todo',
		note: task.goal || null,
		business: task.business || null,
		type: task.type || null,
		size: task.size || null,
		starts_at: instantOf(task.startDate),
		ends_at: instantOf(task.endDate)
	};

	const saved = task.id
		? await client.from('task').update(fields).eq('id', task.id).select('id').single<{ id: string }>()
		: await client
				.from('task')
				.insert({ ...fields, company_id: await companyIDOfMe() })
				.select('id')
				.single<{ id: string }>();
	if (saved.error) throw new Error(saved.error.message);

	await replaceParticipants(saved.data.id, task.participantIDs);
}

export async function deleteSupabaseFlowTask(taskID: string): Promise<void> {
	const { error } = await supabase().from('task').delete().eq('id', taskID);
	if (error) throw new Error(error.message);
}

export async function moveSupabaseFlowTask(taskID: string, status: string): Promise<void> {
	const { error } = await supabase()
		.from('task')
		.update({ status: statusOf[status] ?? 'todo' })
		.eq('id', taskID);
	if (error) throw new Error(error.message);
}

async function replaceParticipants(taskID: string, participantIDs: string[]): Promise<void> {
	const client = supabase();
	const removed = await client.from('task_participant').delete().eq('task_id', taskID);
	if (removed.error) throw new Error(removed.error.message);
	if (participantIDs.length === 0) return;
	const added = await client
		.from('task_participant')
		.insert(participantIDs.map((memberID) => ({ task_id: taskID, member_id: memberID })));
	if (added.error) throw new Error(added.error.message);
}

async function companyIDOfMe(): Promise<string> {
	const client = supabase();
	const { data: auth } = await client.auth.getSession();
	const accountID = auth.session?.user.id;
	if (!accountID) throw new Error('sign in first');
	const member = await client
		.from('member')
		.select('company_id')
		.eq('user_id', accountID)
		.single<{ company_id: string }>();
	if (member.error) throw new Error(member.error.message);
	return member.data.company_id;
}

function displayName(member: MemberRow): string {
	return member.name || (member.email ?? '').split('@')[0];
}

function memberOf(member: MemberRow, tally: MemberTaskTally | undefined): FlowMember {
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

function taskOf(task: TaskRow, nameByID: Map<string, string>): FlowTask {
	const participantIDs = task.task_participant.map((participant) => participant.member_id);
	const endDate = dayOf(task.ends_at ?? task.due_at);
	return {
		id: task.id,
		ownerID: participantIDs[0] ?? '',
		ownerName: nameByID.get(participantIDs[0] ?? '') ?? '',
		participantIDs,
		participantNames: participantIDs.map((memberID) => nameByID.get(memberID) ?? ''),
		business: task.business ?? '',
		type: task.type ?? '',
		content: task.title,
		goal: task.note ?? '',
		size: task.size ?? '',
		status: statusWords[task.status],
		statusRank: 0,
		startDate: dayOf(task.starts_at),
		endDate,
		weekCode: endDate ? weekOf(new Date(`${endDate}T00:00:00Z`)).code : '',
		flag: 0
	};
}

function metricsOf(tasks: FlowTask[]): FlowMetrics {
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
		completedTasks: statusCounts[statusWords.done] ?? 0,
		requestedTasks: 0,
		pausedTasks: statusCounts[statusWords.paused] ?? 0,
		stoppedTasks: statusCounts[statusWords.cancelled] ?? 0,
		statusCounts,
		businessCounts,
		typeCounts
	};
}

function dayOf(instant: string | null): string | undefined {
	return instant ? instant.slice(0, 10) : undefined;
}

function instantOf(day: string | undefined): string | null {
	return day ? new Date(`${day}T00:00:00Z`).toISOString() : null;
}

function weekOf(instant: Date): FlowWeek {
	return weekFromMonday(startOfISOWeek(instant), true);
}

function weekOfCode(code: string): FlowWeek {
	const monday = new Date(`${code}T00:00:00Z`);
	if (Number.isNaN(monday.getTime())) return weekOf(new Date());
	return weekFromMonday(monday, code === weekOf(new Date()).code);
}

function weekFromMonday(monday: Date, isCurrent: boolean): FlowWeek {
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
