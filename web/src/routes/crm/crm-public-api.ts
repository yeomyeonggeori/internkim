import { invokeTool, ToolRefused } from '$lib/public-api-call';
import { CRMApiError } from './crm-error';
import { crmPipelinesOf } from './crm-mappers';
import type {
	CRMOrganizationPayload,
	CRMOrganizationResponse,
	CRMActivityPayload,
	CRMActivityResponse,
	CRMContactPayload,
	CRMContactResponse,
	CRMDataResponse,
	CRMOpportunityPayload,
	CRMOpportunityResponse,
	CRMPositionPayload,
	CRMTransitionPayload,
	CRMVocabulary
} from './crm-api-types';
import {
	activityResponseOf,
	contactResponseOf,
	opportunityResponseOf,
	organizationResponseOf,
	type CRMActivityListToolResult,
	type CRMActivityToolResult,
	type CRMContactListToolResult,
	type CRMContactToolResult,
	type CRMOpportunityListToolResult,
	type CRMOpportunityToolResult,
	type CRMOrganizationListToolResult,
	type CRMOrganizationToolResult,
	type CRMVocabularyToolResult
} from './crm-tool-mappers';

export async function loadSupabaseCRMData(): Promise<CRMDataResponse> {
	const [organizations, contacts, opportunities, activities, vocabulary] = await Promise.all([
		callTool<CRMOrganizationListToolResult>('crm_organization_list', {}),
		callTool<CRMContactListToolResult>('crm_contact_list', {}),
		callTool<CRMOpportunityListToolResult>('crm_opportunity_list', {}),
		callTool<CRMActivityListToolResult>('crm_activity_list', {}),
		callTool<CRMVocabularyToolResult>('crm_vocabulary_get', {})
	]);
	const held: CRMVocabulary = {
		organization_types: vocabulary.organizationTypes,
		pipelines: vocabulary.pipelines
	};
	return {
		organizations: organizations.organizations.map(organizationResponseOf),
		contacts: contacts.contacts.map(contactResponseOf),
		opportunities: opportunities.opportunities.map(opportunityResponseOf),
		activities: activities.activities.map(activityResponseOf),
		pipelines: crmPipelinesOf(held),
		vocabulary: held,
		taskVocabulary: {
			businesses: activities.registeredLabels.businesses,
			types: activities.registeredLabels.types
		}
	};
}

export async function createSupabaseCRMOrganization(payload: CRMOrganizationPayload): Promise<CRMOrganizationResponse> {
	return organizationResponseOf(
		await callTool<CRMOrganizationToolResult>('crm_organization_add', organizationInput(payload))
	);
}

export async function updateSupabaseCRMOrganization(id: string, payload: CRMOrganizationPayload): Promise<CRMOrganizationResponse> {
	return organizationResponseOf(
		await callTool<CRMOrganizationToolResult>('crm_organization_update', {
			organizationHint: id,
			...organizationInput(payload)
		})
	);
}

export async function archiveSupabaseCRMOrganization(id: string): Promise<void> {
	await callTool('crm_organization_archive', { organizationHint: id });
}

export async function createSupabaseCRMContact(payload: CRMContactPayload): Promise<CRMContactResponse> {
	return contactResponseOf(await callTool<CRMContactToolResult>('crm_contact_add', contactInput(payload)));
}

export async function updateSupabaseCRMContact(id: string, payload: CRMContactPayload): Promise<CRMContactResponse> {
	return contactResponseOf(
		await callTool<CRMContactToolResult>('crm_contact_update', {
			contactHint: id,
			...contactInput(payload)
		})
	);
}

export async function archiveSupabaseCRMContact(id: string): Promise<void> {
	await callTool('crm_contact_archive', { contactHint: id });
}

export async function createSupabaseCRMOpportunity(payload: CRMOpportunityPayload): Promise<CRMOpportunityResponse> {
	const transition = requiredTransition(payload.transition);
	return opportunityResponseOf(
		await callTool<CRMOpportunityToolResult>('crm_opportunity_add', {
			...opportunityInput(payload),
			...(settlingStage(transition.stage) ? {} : { stage: transition.stage })
		})
	);
}

export async function updateSupabaseCRMOpportunity(id: string, payload: CRMOpportunityPayload): Promise<CRMOpportunityResponse> {
	const written = await callTool<CRMOpportunityToolResult>('crm_opportunity_update', {
		opportunityHint: id,
		...opportunityInput(payload)
	});
	if (!payload.transition || settlingStage(payload.transition.stage)) return opportunityResponseOf(written);
	return transitionSupabaseCRMOpportunity(id, payload.transition);
}

export async function archiveSupabaseCRMOpportunity(id: string): Promise<void> {
	await callTool('crm_opportunity_archive', { opportunityHint: id });
}

export async function transitionSupabaseCRMOpportunity(id: string, payload: CRMTransitionPayload): Promise<CRMOpportunityResponse> {
	return opportunityResponseOf(
		await callTool<CRMOpportunityToolResult>('crm_opportunity_move', {
			opportunityHint: id,
			stage: payload.stage,
			position: payload.stagePosition,
			closedAt: payload.occurredAt,
			...(payload.lostReason ? { reason: payload.lostReason } : {}),
			...(payload.baseAmountMinor === null ? {} : { finalAmountMinor: payload.baseAmountMinor })
		})
	);
}

export async function positionSupabaseCRMOpportunity(id: string, payload: CRMPositionPayload): Promise<CRMOpportunityResponse> {
	return opportunityResponseOf(
		await callTool<CRMOpportunityToolResult>('crm_opportunity_move', {
			opportunityHint: id,
			position: payload.position
		})
	);
}

export async function createSupabaseCRMActivity(payload: CRMActivityPayload): Promise<CRMActivityResponse> {
	return activityResponseOf(await callTool<CRMActivityToolResult>('crm_activity_save', activityInput(payload)));
}

export async function updateSupabaseCRMActivity(id: string, payload: CRMActivityPayload): Promise<CRMActivityResponse> {
	return activityResponseOf(
		await callTool<CRMActivityToolResult>('crm_activity_save', {
			activityHint: id,
			...activityInput(payload)
		})
	);
}

export async function saveSupabaseCRMVocabulary(vocabulary: CRMVocabulary): Promise<void> {
	await callTool('crm_vocabulary_set', {
		organizationTypes: vocabulary.organization_types,
		pipelines: vocabulary.pipelines
	});
}

async function callTool<Result>(name: string, input: Record<string, unknown>): Promise<Result> {
	try {
		return await invokeTool<Result>(name, input);
	} catch (refusal) {
		if (refusal instanceof ToolRefused) {
			throw new CRMApiError(refusal.message, refusal.status, refusal.errorCode ?? 'request_failed');
		}
		throw refusal;
	}
}

function organizationInput(payload: CRMOrganizationPayload): Record<string, unknown> {
	return {
		name: payload.name,
		status: payload.status,
		types: payload.types,
		tags: payload.tags,
		importance: payload.importance,
		ownerPersonHint: payload.ownerPersonID ?? '',
		address: payload.address ?? '',
		description: payload.description ?? ''
	};
}

function contactInput(payload: CRMContactPayload): Record<string, unknown> {
	return {
		name: payload.name,
		...(payload.organizationID ? { organizationHint: payload.organizationID } : {}),
		email: payload.email ?? '',
		phoneNumber: payload.phone ?? '',
		role: payload.title ?? '',
		department: payload.department ?? '',
		description: payload.description ?? ''
	};
}

function opportunityInput(payload: CRMOpportunityPayload): Record<string, unknown> {
	return {
		organizationHint: payload.organizationID,
		title: payload.name,
		pipeline: payload.pipeline,
		business: payload.business ?? '',
		importance: payload.importance,
		description: payload.description ?? '',
		ownerPersonHint: payload.ownerPersonID ?? '',
		contactHint: payload.contacts[0]?.contactID ?? '',
		currencyCode: payload.currencyCode || '',
		expectedCloseDate: payload.dueAt ?? '',
		...(payload.amountMinor === null ? {} : { amountMinor: payload.amountMinor })
	};
}

function activityInput(payload: CRMActivityPayload): Record<string, unknown> {
	return {
		organizationHint: payload.organizationID,
		opportunityHint: payload.opportunityID ?? '',
		contactHint: payload.contactID ?? '',
		title: payload.title,
		kind: payload.kind,
		business: payload.business ?? '',
		note: payload.content ?? '',
		status: payload.taskStatus || 'todo',
		occurredAt: payload.occurredAt,
		ownerPersonHint: payload.taskOwnerID ?? '',
		isEvent: payload.isEvent,
		isWholeDay: payload.isEvent && payload.isWholeDay,
		...(payload.isEvent ? { startsAt: payload.startsAt || payload.occurredAt } : {}),
		...(payload.isEvent
			? { endsAt: payload.endsAt || payload.startsAt || payload.occurredAt }
			: {}),
		...(payload.isEvent && payload.location ? { location: payload.location } : {}),
		...(payload.isEvent && payload.notifyMinutesBefore !== null
			? { notifyMinutesBefore: payload.notifyMinutesBefore }
			: {})
	};
}

function settlingStage(stage: string): boolean {
	return stage === 'done' || stage === 'lost';
}

function requiredTransition(value: CRMTransitionPayload | undefined): CRMTransitionPayload {
	if (value) return value;
	throw new CRMApiError('진행 단계가 필요합니다.', 400, 'stage_required');
}
