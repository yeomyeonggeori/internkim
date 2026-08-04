import { supabase } from '$lib/supabase';
import { flowDefinitionsOf, vocabularyOf } from '$lib/flow/task-vocabulary';
import type {
	FlowMember,
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
	task_participant: { member_id: string }[];
};

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

	const tasks = await client
		.from('task')
		.select('id, title, status, note, business, type, size, starts_at, ends_at, due_at, task_participant (member_id)')
		.eq('is_event', false)
		.order('ends_at', { ascending: false, nullsFirst: false })
		.returns<TaskRow[]>();
	if (tasks.error) throw new Error(tasks.error.message);

	const nameByID = new Map(members.data.map((member) => [member.id, displayName(member)]));
	const flowTasks = tasks.data.map((task) => taskOf(task, nameByID));
	const me = members.data.find((member) => member.user_id === accountID);

	return {
		currentWeek: weekOf(new Date()),
		members: members.data.map((member) => memberOf(member, flowTasks)),
		tasks: flowTasks,
		metrics: metricsOf(flowTasks),
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

function memberOf(member: MemberRow, tasks: FlowTask[]): FlowMember {
	const mine = tasks.filter((task) => task.participantIDs.includes(member.id));
	return {
		id: member.id,
		name: displayName(member),
		email: member.email ?? '',
		hireDate: member.joined_at ? member.joined_at.slice(0, 10) : undefined,
		role: member.is_admin ? 'admin' : 'member',
		mattermostStatus: '',
		activeTaskCount: mine.filter((task) => task.status === statusWords.in_progress).length,
		completeTaskCount: mine.filter((task) => task.status === statusWords.done).length
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
	const monday = new Date(Date.UTC(instant.getUTCFullYear(), instant.getUTCMonth(), instant.getUTCDate()));
	monday.setUTCDate(monday.getUTCDate() - ((monday.getUTCDay() + 6) % 7));
	return weekFromMonday(monday, true);
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
