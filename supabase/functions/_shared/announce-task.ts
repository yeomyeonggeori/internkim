import type { SupabaseClient } from './service-client.ts';
import { centralTaskStatuses, type CentralTaskStatus } from './central-task-status.ts';
import { colleaguesOfCompany } from './company-colleagues.ts';
import { pictureURLOfMember } from './member-directory.ts';
import { notifyMember, type Notification } from './notify-member.ts';
import type { PushKeys } from './push-keys.ts';
import { whoHearsTheCompletionOf } from './task-audience.ts';

type TaskRow = {
	id: string;
	title: string;
	status: string;
	company_id: string;
	requester_id: string | null;
	task_participant: { member_id: string }[];
};

export type Announced = { told: number; reached: number };

export function whoTaskMoveConcerns(task: TaskRow, moverID: string): string[] {
	const concerned = new Set<string>();
	if (task.requester_id) concerned.add(task.requester_id);
	for (const participant of task.task_participant ?? []) concerned.add(participant.member_id);
	concerned.delete(moverID);
	return [...concerned];
}

export type TaskChange = { taskID: string; moved: boolean; askedMemberIDs: string[] };

export function whoTaskWasAskedOf(task: TaskRow, askedMemberIDs: string[], moverID: string): string[] {
	const participants = new Set((task.task_participant ?? []).map((participant) => participant.member_id));
	return [...new Set(askedMemberIDs)].filter((memberID) => memberID !== moverID && participants.has(memberID));
}

export async function announceTaskChange(
	caller: SupabaseClient,
	record: SupabaseClient,
	moverID: string,
	change: TaskChange,
	pushKeys: PushKeys,
	nowInSeconds: number
): Promise<Announced> {
	const task = await taskOf(caller, change.taskID);
	if (!task) return { told: 0, reached: 0 };

	const [mover, moverPicture] = await Promise.all([
		nameOfMember(record, moverID),
		pictureURLOfMember(record, moverID)
	]);
	const fromMover = { openPath: '/task/', tag: `task-${task.id}`, senderID: moverID, senderName: mover, icon: moverPicture };
	const asked = whoTaskWasAskedOf(task, change.askedMemberIDs, moverID);
	const askedAnnounced = await tellEach(
		record,
		asked,
		{ ...fromMover, title: task.title, body: `${mover}님이 업무를 요청했습니다` },
		pushKeys,
		nowInSeconds
	);
	if (!change.moved) return askedAnnounced;

	const concerned = whoTaskMoveConcerns(task, moverID);
	const told = new Set([...asked, ...concerned]);
	const moveAnnounced = await tellEach(
		record,
		[
			...concerned.filter((memberID) => !asked.includes(memberID)),
			...(await whoElseHearsTheMove(record, task, moverID)).filter((memberID) => !told.has(memberID))
		],
		{ ...fromMover, title: `${centralStatusWord(task.status)}: ${task.title}`, body: `${mover}님이 옮겼습니다` },
		pushKeys,
		nowInSeconds
	);
	return {
		told: askedAnnounced.told + moveAnnounced.told,
		reached: askedAnnounced.reached + moveAnnounced.reached
	};
}

const statusColleaguesHearOf: CentralTaskStatus = 'completed';

async function whoElseHearsTheMove(record: SupabaseClient, task: TaskRow, moverID: string): Promise<string[]> {
	if (task.status !== statusColleaguesHearOf) return [];
	const colleagues = await colleaguesOfCompany(record, task.company_id);
	return whoHearsTheCompletionOf(
		moverID,
		colleagues.map((colleague) => ({
			id: colleague.id,
			teamID: colleague.teamID,
			supervisorID: colleague.supervisorID,
			taskReach: colleague.notificationSettings.taskReach
		}))
	);
}

function centralStatusWord(status: string): string {
	if (!centralTaskStatuses.includes(status as (typeof centralTaskStatuses)[number])) {
		throw new Error(`unsupported task status: ${status}`);
	}
	return status;
}

async function taskOf(caller: SupabaseClient, taskID: string): Promise<TaskRow | null> {
	const { data, error } = await caller
		.from('task')
		.select('id, title, status, company_id, requester_id, task_participant (member_id)')
		.eq('id', taskID)
		.maybeSingle<TaskRow>();
	if (error) throw new Error(error.message);
	return data;
}

async function nameOfMember(record: SupabaseClient, memberID: string): Promise<string> {
	const { data, error } = await record
		.from('member')
		.select('name')
		.eq('id', memberID)
		.maybeSingle<{ name: string | null }>();
	if (error) throw new Error(error.message);
	return (data?.name ?? '').trim() || '누군가';
}

async function tellEach(
	record: SupabaseClient,
	memberIDs: string[],
	notification: Notification,
	pushKeys: PushKeys,
	nowInSeconds: number
): Promise<Announced> {
	let told = 0;
	let reached = 0;
	for (const memberID of memberIDs) {
		const delivery = await notifyMember(record, memberID, 'task', notification, pushKeys, nowInSeconds);
		if (!delivery.silent) told += 1;
		reached += delivery.reached;
	}
	return { told, reached };
}
