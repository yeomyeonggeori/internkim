import type { SupabaseClient } from './service-client.ts';
import { centralTaskStatuses } from './central-task-status.ts';
import { pictureURLOfMember } from './member-directory.ts';
import { notifyMember, type Notification } from './notify-member.ts';
import type { PushKeys } from './push-keys.ts';

type TaskRow = {
	id: string;
	title: string;
	status: string;
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

export async function announceTaskMove(
	caller: SupabaseClient,
	record: SupabaseClient,
	moverID: string,
	taskID: string,
	pushKeys: PushKeys,
	nowInSeconds: number
): Promise<Announced> {
	const task = await taskOf(caller, taskID);
	if (!task) return { told: 0, reached: 0 };

	const [mover, moverPicture] = await Promise.all([
		nameOfMember(record, moverID),
		pictureURLOfMember(record, moverID)
	]);
	const notification: Notification = {
		title: `${centralStatusWord(task.status)}: ${task.title}`,
		body: `${mover}님이 옮겼습니다`,
		openPath: '/task/',
		tag: `task-${task.id}`,
		senderName: mover,
		icon: moverPicture
	};
	return tellEach(record, whoTaskMoveConcerns(task, moverID), notification, pushKeys, nowInSeconds);
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
		.select('id, title, status, requester_id, task_participant (member_id)')
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
