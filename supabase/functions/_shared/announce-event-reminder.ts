import type { SupabaseClient } from './service-client.ts';
import {
	isDue,
	spellOutMinutes,
	startsIn,
	statusesStillAhead,
	whoStillListens,
	type DueEvent,
	type Participant
} from './event-reminder-rules.ts';
import { notifyMember, type Notification } from './notify-member.ts';
import type { PushKeys } from './push-keys.ts';

type EventRow = {
	id: string;
	title: string;
	starts_at: string;
	is_whole_day: boolean;
	notify_minutes_before: number | null;
	task_participant: Participant[];
};

export type Announced = { told: number; reached: number };

export async function announceEventReminders(
	record: SupabaseClient,
	moment: Date,
	pushKeys: PushKeys,
	nowInSeconds: number
): Promise<Announced> {
	let told = 0;
	let reached = 0;
	for (const event of await eventsWithAReminder(record, moment)) {
		if (!isDue(asDueEvent(event), moment)) continue;
		const notification = reminderNotification(event, moment);
		for (const memberID of whoStillListens(event.task_participant)) {
			const delivery = await notifyMember(record, memberID, 'calendar', notification, pushKeys, nowInSeconds);
			if (!delivery.silent) told += 1;
			reached += delivery.reached;
		}
	}
	return { told, reached };
}

async function eventsWithAReminder(record: SupabaseClient, moment: Date): Promise<EventRow[]> {
	const { data, error } = await record
		.from('task')
		.select(
			'id, title, starts_at, is_whole_day, notify_minutes_before, task_participant (member (id, status))'
		)
		.eq('is_event', true)
		.in('status', statusesStillAhead)
		.not('notify_minutes_before', 'is', null)
		.gt('starts_at', moment.toISOString())
		.returns<EventRow[]>();
	if (error) throw new Error(error.message);
	return data ?? [];
}

function asDueEvent(event: EventRow): DueEvent {
	return {
		id: event.id,
		title: event.title,
		startsAt: event.starts_at,
		isWholeDay: event.is_whole_day,
		notifyMinutesBefore: event.notify_minutes_before
	};
}

function reminderNotification(event: EventRow, moment: Date): Notification {
	return {
		title: event.title,
		body: howSoon(asDueEvent(event), moment),
		openPath: '/calendar/',
		tag: `event-reminder-${event.id}`
	};
}

function howSoon(event: DueEvent, moment: Date): string {
	if (event.isWholeDay) return '종일 일정';
	return `${spellOutMinutes(startsIn(event, moment))} 뒤 시작`;
}
