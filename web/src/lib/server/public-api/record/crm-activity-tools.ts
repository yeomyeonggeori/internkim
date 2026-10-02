import { opportunityOfCRMHint, organizationOfCRMHint } from './crm-tools';
import { eventAdd, eventUpdate, locationNameOf } from './event-tools';
import { taskAdd, taskUpdate } from './task-tools';
import { taskOfHint, tasksOfCompany, type TaskReadScope, type TaskRow } from './tasks';
import { WorkspaceTaskSize, WorkspaceTaskStatus } from '../catalog/tools';
import type { RecordContext } from './company';
import type { CRMActivityListResult, CRMActivityResult, CRMActivitySaveResult } from '../catalog/crm';

export type CRMActivityListInput = {
	organizationHint?: string;
	opportunityHint?: string;
	query?: string;
};

export type CRMActivitySaveInput = {
	activityHint?: string;
	organizationHint?: string;
	opportunityHint?: string;
	contactHint?: string;
	title?: string;
	kind?: string;
	business?: string;
	note?: string;
	status?: string;
	occurredAt?: string;
	participantPersonHints?: string[];
	ownerPersonHint?: string;
	isEvent?: boolean;
	isWholeDay?: boolean;
	startsAt?: string;
	endsAt?: string;
	location?: string;
	notifyMinutesBefore?: number;
};

function participantHintsOf(input: CRMActivitySaveInput): string[] | undefined {
	if (input.participantPersonHints?.length) return input.participantPersonHints;
	if (input.ownerPersonHint) return [input.ownerPersonHint];
	return undefined;
}

function answeredActivity(row: TaskRow): CRMActivityResult {
	return {
		activityID: row.id,
		organizationID: row.organization_id ?? '',
		opportunityID: row.opportunity_id ?? '',
		contactID: row.contact_id ?? '',
		business: row.business ?? '',
		kind: row.type || 'task',
		title: row.title,
		occurredAt: row.starts_at ?? row.due_at ?? row.created_at,
		content: row.note ?? '',
		taskStatus: row.status,
		size: row.size ?? '',
		participantIDs: row.task_participant.map((participant) => participant.member_id),
		ownerPersonID: row.task_participant[0]?.member_id ?? '',
		requesterPersonID: row.requester_id ?? '',
		isEvent: row.is_event,
		isWholeDay: row.is_whole_day,
		startsAt: row.starts_at ?? '',
		endsAt: row.ends_at ?? '',
		notifyMinutesBefore: row.notify_minutes_before,
		location: locationNameOf(row.location),
		createdAt: row.created_at,
		updatedAt: row.updated_at
	};
}

async function activitiesOfCompany(context: RecordContext, scope: TaskReadScope = {}): Promise<TaskRow[]> {
	const linked = { ...scope, linkedToOrganization: true };
	const [tasks, events] = await Promise.all([
		tasksOfCompany(context.caller, false, linked),
		tasksOfCompany(context.caller, true, linked)
	]);
	return [...tasks, ...events]
		.sort((left, right) => right.created_at.localeCompare(left.created_at));
}

function searchableText(value: string): string {
	return value.trim().toLowerCase().split(/\s+/).join('');
}

export async function crmActivityList(
	context: RecordContext,
	input: CRMActivityListInput
): Promise<CRMActivityListResult> {
	const organizationID = input.organizationHint
		? (await organizationOfCRMHint(context, input.organizationHint)).id
		: '';
	const opportunityID = input.opportunityHint
		? (await opportunityOfCRMHint(context, input.opportunityHint)).id
		: '';
	const asked = searchableText(input.query ?? '');

	const rows = (await activitiesOfCompany(context, { organizationID, opportunityID })).filter((row) => {
		if (!asked) return true;
		return [row.title, row.note ?? ''].some((value) => searchableText(value).includes(asked));
	});

	return {
		count: rows.length,
		activities: rows.map(answeredActivity),
		registeredLabels: {
			businesses: context.labels.businesses,
			types: context.labels.types,
			sizes: Object.values(WorkspaceTaskSize),
			statuses: Object.values(WorkspaceTaskStatus)
		}
	};
}

async function activityRowOfHint(context: RecordContext, hint: string): Promise<TaskRow> {
	return taskOfHint(await activitiesOfCompany(context), hint, 'task', context.requesterID);
}

async function activityByID(context: RecordContext, activityID: string): Promise<TaskRow> {
	const held = (await activitiesOfCompany(context)).find((row) => row.id === activityID);
	if (!held) throw new Error('the record saved this activity and did not answer with it');
	return held;
}

const stageChangeKind = 'stage_change';

export async function crmActivitySave(
	context: RecordContext,
	input: CRMActivitySaveInput
): Promise<CRMActivitySaveResult> {
	const held = input.activityHint ? await activityRowOfHint(context, input.activityHint) : null;
	if (!held && !input.organizationHint?.trim()) {
		throw new Error('an activity names the organization it is with');
	}
	if (!held && input.kind?.trim() === stageChangeKind) {
		throw new Error('a stage change is recorded by moving the deal, not by writing an activity');
	}
	const links = {
		...(input.organizationHint !== undefined ? { organizationHint: input.organizationHint } : {}),
		...(input.opportunityHint !== undefined ? { opportunityHint: input.opportunityHint } : {}),
		...(input.contactHint !== undefined ? { contactHint: input.contactHint } : {})
	};
	const written = {
		...links,
		...(input.title !== undefined ? { title: input.title } : {}),
		...(input.business !== undefined ? { business: input.business } : {}),
		...(input.kind !== undefined && input.kind !== stageChangeKind ? { type: input.kind } : {}),
		...(input.note !== undefined ? { note: input.note } : {}),
		...(participantHintsOf(input) ? { participantPersonHints: participantHintsOf(input) } : {})
	};

	if (input.isEvent) {
		const answered = held
			? await eventUpdate(context, { ...written, ...eventFields(input), eventHint: held.id })
			: await eventAdd(context, { ...written, ...eventFields(input) });
		return { ...answeredActivity(await activityByID(context, answered.eventID)), isNew: !held };
	}

	const asATask = {
		...written,
		...(input.status !== undefined ? { status: input.status } : {}),
		...(input.occurredAt !== undefined ? { dueAt: input.occurredAt } : {})
	};
	const answered = held
		? await taskUpdate(context, { ...asATask, taskHint: held.id })
		: await taskAdd(context, asATask);
	return { ...answeredActivity(await activityByID(context, answered.taskID)), isNew: !held };
}

function eventFields(input: CRMActivitySaveInput) {
	const startsAt = input.startsAt || input.occurredAt;
	return {
		...(startsAt ? { startsAt } : {}),
		...(input.endsAt || startsAt ? { endsAt: input.endsAt || startsAt } : {}),
		...(input.isWholeDay !== undefined ? { isWholeDay: input.isWholeDay } : {}),
		...(input.location !== undefined ? { location: input.location } : {}),
		...(input.notifyMinutesBefore !== undefined
			? { notifyMinutesBefore: input.notifyMinutesBefore }
			: {})
	};
}
