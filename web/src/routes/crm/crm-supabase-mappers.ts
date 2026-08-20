import type {
	CRMAuditResponse,
	CRMOrganizationResponse,
	CRMActivityResponse,
	CRMContactResponse,
	CRMDataResponse,
	CRMLostReasonResponse,
	CRMOpportunityResponse,
	CRMPipelineResponse,
	CRMPipelineStageResponse,
	CRMVocabulary
} from './crm-api-types';
import type {
	CRMOrganizationStatus,
	CRMOrganizationType,
	CRMActivityKind,
	CRMCurrency,
	CRMImportance,
	CRMProgressKind
} from './crm-types';
import { vocabularyOf } from '$lib/flow/task-vocabulary';

export type CRMAuditRow = {
	created_at: string;
	created_by: string | null;
	updated_at: string;
	updated_by: string | null;
	archived_at: string | null;
	archived_by: string | null;
};

export type OrganizationRow = CRMAuditRow & {
	id: string;
	name: string;
	status: string;
	types: string[];
	tags: string[];
	importance: string;
	owner_id: string | null;
	address: string | null;
	description: string | null;
};

export type ContactRow = CRMAuditRow & {
	id: string;
	organization_id: string | null;
	name: string;
	email: string | null;
	phone: string | null;
	title: string | null;
	department: string | null;
	description: string | null;
};

export type OpportunityRow = CRMAuditRow & {
	id: string;
	organization_id: string;
	contact_id: string | null;
	name: string;
	business: string | null;
	pipeline_id: string;
	stage_id: string;
	stage_position: number;
	stage_changed_at: string;
	owner_id: string | null;
	amount_minor: number | null;
	currency_code: string | null;
	base_amount_minor: number | null;
	base_currency_code: string | null;
	importance: string;
	due_at: string | null;
	due_time_zone: string | null;
	lost_reason_id: string | null;
	description: string | null;
};

export type CRMTaskRow = {
	id: string;
	organization_id: string;
	opportunity_id: string | null;
	contact_id: string | null;
	title: string;
	note: string | null;
	business: string | null;
	type: string | null;
	due_at: string | null;
	starts_at: string | null;
	created_at: string | null;
	updated_at: string;
	requester_id: string | null;
	status: string;
	is_event: boolean;
	is_whole_day: boolean;
	ends_at: string | null;
	notify_minutes_before: number | null;
	location: { name?: string } | null;
	task_participant: { member_id: string }[];
};

export function crmDataResponseOf(
	organizations: OrganizationRow[],
	contacts: ContactRow[],
	opportunities: OpportunityRow[],
	tasks: CRMTaskRow[],
	vocabularyValue: unknown,
	taskVocabularyValue: unknown = {}
): CRMDataResponse {
	const vocabulary = crmVocabularyOf(vocabularyValue);
	return {
		organizations: organizations.filter(active).map(organizationOf),
		contacts: contacts.filter(active).map(contactOf),
		opportunities: opportunities.filter(active).map(opportunityOf),
		activities: tasks.map(activityOf),
		pipelines: pipelineResponses(vocabulary),
		stages: stageResponses(vocabulary),
		lostReasons: lostReasonResponses(vocabulary),
		vocabulary,
		taskVocabulary: vocabularyOf(taskVocabularyValue)
	};
}

export function crmVocabularyOf(value: unknown): CRMVocabulary {
	if (!isRecord(value)) return emptyVocabulary();
	return {
		organization_types: definitionArray(value.organization_types),
		pipelines: pipelineArray(value.pipelines),
		lost_reasons: definitionArray(value.lost_reasons)
	};
}

function organizationOf(row: OrganizationRow): CRMOrganizationResponse {
	return {
		id: row.id,
		name: row.name,
		status: organizationStatusOf(row.status),
		types: row.types as CRMOrganizationType[],
		tags: row.tags,
		importance: importanceOf(row.importance),
		ownerPersonID: row.owner_id ?? '',
		address: row.address ?? undefined,
		description: row.description ?? undefined,
		audit: auditOf(row)
	};
}

function contactOf(row: ContactRow): CRMContactResponse {
	return {
		id: row.id,
		organizationID: row.organization_id ?? '',
		name: row.name,
		email: row.email ?? undefined,
		phone: row.phone ?? undefined,
		title: row.title ?? undefined,
		department: row.department ?? undefined,
		ownerPersonID: '',
		description: row.description ?? undefined,
		audit: auditOf(row)
	};
}

function opportunityOf(row: OpportunityRow): CRMOpportunityResponse {
	return {
		id: row.id,
		organizationID: row.organization_id,
		business: row.business ?? undefined,
		name: row.name,
		pipeline: row.pipeline_id as CRMProgressKind,
		stage: row.stage_id,
		stagePosition: row.stage_position,
		stageChangedAt: row.stage_changed_at,
		ownerPersonID: row.owner_id ?? '',
		amountMinor: row.amount_minor ?? undefined,
		currencyCode: currencyOf(row.currency_code),
		baseAmountMinor: row.base_amount_minor ?? undefined,
		baseCurrencyCode: optionalCurrencyOf(row.base_currency_code),
		importance: importanceOf(row.importance),
		dueAt: row.due_at ?? undefined,
		dueTimeZone: row.due_time_zone ?? undefined,
		lostReason: row.lost_reason_id ?? undefined,
		description: row.description ?? undefined,
		contacts: row.contact_id ? [{ contactID: row.contact_id }] : [],
		audit: auditOf(row)
	};
}

function activityOf(row: CRMTaskRow): CRMActivityResponse {
	return {
		id: row.id,
		organizationID: row.organization_id,
		contactID: row.contact_id ?? undefined,
		opportunityID: row.opportunity_id ?? undefined,
		business: row.business ?? undefined,
		kind: (row.type || 'task') as CRMActivityKind,
		title: row.title,
		occurredAt: row.starts_at ?? row.due_at ?? row.created_at ?? row.updated_at,
		content: row.note ?? undefined,
		taskStatus: row.status,
		taskOwnerID: row.task_participant[0]?.member_id,
		isEvent: row.is_event,
		isWholeDay: row.is_whole_day,
		startsAt: row.starts_at ?? undefined,
		endsAt: row.ends_at ?? undefined,
		notifyMinutesBefore: row.notify_minutes_before ?? undefined,
		location: row.location?.name,
		audit: {
			createdAt: row.created_at ?? row.updated_at,
			createdByPersonID: row.requester_id ?? '',
			updatedAt: row.updated_at,
			updatedByPersonID: row.requester_id ?? ''
		}
	};
}

function auditOf(row: CRMAuditRow): CRMAuditResponse {
	return {
		createdAt: row.created_at,
		createdByPersonID: row.created_by ?? '',
		updatedAt: row.updated_at,
		updatedByPersonID: row.updated_by ?? '',
		archivedAt: row.archived_at ?? undefined,
		archivedByPersonID: row.archived_by ?? undefined
	};
}

function pipelineResponses(vocabulary: CRMVocabulary): CRMPipelineResponse[] {
	return vocabulary.pipelines.map((pipeline) => ({
		pipeline: pipeline.id as CRMProgressKind,
		label: pipeline.name,
		direction: pipeline.direction ?? '',
		isActive: true
	}));
}

function stageResponses(vocabulary: CRMVocabulary): CRMPipelineStageResponse[] {
	return vocabulary.pipelines.flatMap((pipeline) => pipeline.stages.map((stage, position) => ({
		pipeline: pipeline.id as CRMProgressKind,
		stage: stage.id,
		label: stage.name,
		position,
		outcome: stage.outcome
	})));
}

function lostReasonResponses(vocabulary: CRMVocabulary): CRMLostReasonResponse[] {
	return vocabulary.lost_reasons.map((reason) => ({ reason: reason.id, label: reason.name, isActive: true }));
}

function active(row: CRMAuditRow): boolean {
	return row.archived_at === null;
}

function organizationStatusOf(value: string): CRMOrganizationStatus {
	return value === 'prospect' || value === 'paused' ? value : 'active';
}

function importanceOf(value: string): CRMImportance {
	return value === 'high' || value === 'low' ? value : 'medium';
}

const isoCurrencyCodePattern = /^[A-Z]{3}$/;

function currencyOf(value: string | null): CRMCurrency | '' {
	return value !== null && isoCurrencyCodePattern.test(value) ? value : '';
}

function optionalCurrencyOf(value: string | null): CRMCurrency | undefined {
	const currency = currencyOf(value);
	return currency || undefined;
}

function emptyVocabulary(): CRMVocabulary {
	return { organization_types: [], pipelines: [], lost_reasons: [] };
}

function definitionArray(value: unknown): Array<{ id: string; name: string; color?: string }> {
	if (!Array.isArray(value)) return [];
	return value.flatMap((item) => {
		if (!isRecord(item) || typeof item.id !== 'string' || typeof item.name !== 'string') return [];
		return [{ id: item.id, name: item.name, color: typeof item.color === 'string' ? item.color : undefined }];
	});
}

function pipelineArray(value: unknown): CRMVocabulary['pipelines'] {
	if (!Array.isArray(value)) return [];
	return value.flatMap((item) => {
		if (!isRecord(item) || typeof item.id !== 'string' || typeof item.name !== 'string') return [];
		const stages = Array.isArray(item.stages) ? item.stages.flatMap(stageOf) : [];
		return [{ id: item.id, name: item.name, direction: typeof item.direction === 'string' ? item.direction : undefined, stages }];
	});
}

function stageOf(value: unknown): CRMVocabulary['pipelines'][number]['stages'] {
	if (!isRecord(value) || typeof value.id !== 'string' || typeof value.name !== 'string') return [];
	const outcome = value.outcome;
	if (outcome !== 'open' && outcome !== 'won' && outcome !== 'lost' && outcome !== 'on_hold') return [];
	return [{ id: value.id, name: value.name, outcome, color: typeof value.color === 'string' ? value.color : undefined }];
}

function isRecord(value: unknown): value is Record<string, unknown> {
	return typeof value === 'object' && value !== null && !Array.isArray(value);
}
