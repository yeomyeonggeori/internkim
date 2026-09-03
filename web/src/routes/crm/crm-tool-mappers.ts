import { isCRMStage } from '$lib/crm/crm-stage';
import { CRMApiError } from './crm-api';
import type {
	CRMAuditResponse,
	CRMContactResponse,
	CRMOpportunityResponse,
	CRMOrganizationResponse
} from './crm-api-types';
import type {
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
