import { isCRMStage } from '$lib/crm/crm-stage';
import { CRMApiError } from './crm-api';
import type {
	CRMActivityResponse,
	CRMAuditResponse,
	CRMContactResponse,
	CRMOpportunityResponse,
	CRMOrganizationResponse
} from './crm-api-types';
import type { NamedColour } from '$lib/task/task-vocabulary';
import type {
	CRMActivityKind,
	CRMCurrency,
	CRMImportance,
	CRMOpportunityStage,
	CRMOrganizationStatus,
	CRMOrganizationType,
	CRMProgressKind
} from './crm-types';

type CRMAuditToolResult = {
	createdAt: string;
	createdByPersonID: string;
	updatedAt: string;
	updatedByPersonID: string;
	archivedAt: string | null;
	archivedByPersonID: string;
};

export type CRMOrganizationToolResult = {
	organizationID: string;
	name: string;
	status: string;
	types: string[];
	tags: string[];
	importance: string;
	ownerPersonID: string;
	address: string;
	description: string;
	audit: CRMAuditToolResult;
};

export type CRMContactToolResult = {
	contactID: string;
	organizationID: string;
	name: string;
	email: string;
	phoneNumber: string;
	role: string;
	department: string;
	description: string;
	audit: CRMAuditToolResult;
};

export type CRMOpportunityToolResult = {
	opportunityID: string;
	organizationID: string;
	contactID: string;
	title: string;
	business: string;
	pipeline: string;
	stage: string;
	stagePosition: number;
	stageChangedAt: string;
	ownerPersonID: string;
	amountMinor: number | null;
	currencyCode: string;
	baseAmountMinor: number | null;
	baseCurrencyCode: string;
	importance: string;
	expectedCloseAt: string;
	expectedCloseTimeZone: string;
	lostReason: string;
	description: string;
	activityCount: number;
	audit: CRMAuditToolResult;
};

const organizationStatuses: CRMOrganizationStatus[] = ['prospect', 'active', 'paused'];
const importances: CRMImportance[] = ['high', 'medium', 'low'];

export function organizationResponseOf(result: CRMOrganizationToolResult): CRMOrganizationResponse {
	return {
		id: result.organizationID,
		name: result.name,
		status: statusOf(result.status),
		types: result.types as CRMOrganizationType[],
		tags: result.tags,
		importance: importanceOf(result.importance),
		ownerPersonID: result.ownerPersonID,
		address: result.address || undefined,
		description: result.description || undefined,
		audit: auditOf(result.audit)
	};
}

export function contactResponseOf(result: CRMContactToolResult): CRMContactResponse {
	return {
		id: result.contactID,
		organizationID: result.organizationID,
		name: result.name,
		email: result.email || undefined,
		phone: result.phoneNumber || undefined,
		title: result.role || undefined,
		department: result.department || undefined,
		ownerPersonID: '',
		description: result.description || undefined,
		audit: auditOf(result.audit)
	};
}

export function opportunityResponseOf(result: CRMOpportunityToolResult): CRMOpportunityResponse {
	return {
		id: result.opportunityID,
		organizationID: result.organizationID,
		business: result.business || undefined,
		name: result.title,
		pipeline: result.pipeline as CRMProgressKind,
		stage: stageOf(result.stage),
		stagePosition: result.stagePosition,
		stageChangedAt: result.stageChangedAt,
		ownerPersonID: result.ownerPersonID,
		amountMinor: result.amountMinor ?? undefined,
		currencyCode: currencyOf(result.currencyCode),
		baseAmountMinor: result.baseAmountMinor ?? undefined,
		baseCurrencyCode: currencyOf(result.baseCurrencyCode) || undefined,
		importance: importanceOf(result.importance),
		dueAt: result.expectedCloseAt || undefined,
		dueTimeZone: result.expectedCloseTimeZone || undefined,
		lostReason: result.lostReason || undefined,
		description: result.description || undefined,
		contacts: result.contactID ? [{ contactID: result.contactID }] : [],
		audit: auditOf(result.audit)
	};
}

function auditOf(audit: CRMAuditToolResult): CRMAuditResponse {
	return {
		createdAt: audit.createdAt,
		createdByPersonID: audit.createdByPersonID,
		updatedAt: audit.updatedAt,
		updatedByPersonID: audit.updatedByPersonID,
		archivedAt: audit.archivedAt ?? undefined,
		archivedByPersonID: audit.archivedByPersonID || undefined
	};
}

function stageOf(value: string): CRMOpportunityStage {
	if (isCRMStage(value)) return value;
	throw new CRMApiError(`${value} is not a stage this CRM knows`, 502, 'invalid_response');
}

function statusOf(value: string): CRMOrganizationStatus {
	return organizationStatuses.find((status) => status === value) ?? 'active';
}

function importanceOf(value: string): CRMImportance {
	return importances.find((importance) => importance === value) ?? 'medium';
}

function currencyOf(value: string): CRMCurrency {
	return /^[A-Z]{3}$/.test(value) ? value : '';
}

export type CRMActivityToolResult = {
	activityID: string;
	organizationID: string;
	opportunityID: string;
	contactID: string;
	business: string;
	kind: string;
	title: string;
	occurredAt: string;
	content: string;
	taskStatus: string;
	ownerPersonID: string;
	requesterPersonID: string;
	isEvent: boolean;
	isWholeDay: boolean;
	startsAt: string;
	endsAt: string;
	notifyMinutesBefore: number | null;
	location: string;
	createdAt: string;
	updatedAt: string;
};

export type CRMOrganizationListToolResult = { count: number; organizations: CRMOrganizationToolResult[] };
export type CRMContactListToolResult = { count: number; contacts: CRMContactToolResult[] };
export type CRMOpportunityListToolResult = { count: number; opportunities: CRMOpportunityToolResult[] };
export type CRMActivityListToolResult = {
	count: number;
	activities: CRMActivityToolResult[];
	registeredLabels: { businesses: NamedColour[]; types: NamedColour[]; sizes: string[]; statuses: string[] };
};
export type CRMVocabularyToolResult = {
	organizationTypes: { id: string; name: string; color?: string }[];
	pipelines: { id: string; name: string; color?: string; direction?: string }[];
	stages: { stage: string; outcome: string }[];
};

export function activityResponseOf(result: CRMActivityToolResult): CRMActivityResponse {
	return {
		id: result.activityID,
		organizationID: result.organizationID,
		contactID: result.contactID || undefined,
		opportunityID: result.opportunityID || undefined,
		business: result.business || undefined,
		kind: (result.kind || 'task') as CRMActivityKind,
		title: result.title,
		occurredAt: result.occurredAt,
		content: result.content || undefined,
		taskStatus: result.taskStatus,
		taskOwnerID: result.ownerPersonID || undefined,
		isEvent: result.isEvent,
		isWholeDay: result.isWholeDay,
		startsAt: result.startsAt || undefined,
		endsAt: result.endsAt || undefined,
		notifyMinutesBefore: result.notifyMinutesBefore ?? undefined,
		location: result.location || undefined,
		audit: {
			createdAt: result.createdAt,
			createdByPersonID: result.requesterPersonID,
			updatedAt: result.updatedAt,
			updatedByPersonID: result.requesterPersonID
		}
	};
}
