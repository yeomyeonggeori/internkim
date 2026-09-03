import { supabase } from '$lib/supabase';
import { invokeTool, ToolRefused } from '$lib/public-api-call';
import { CRMApiError } from './crm-api';
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
	crmDataResponseOf,
	type ContactRow,
	type CRMTaskRow,
	type OpportunityRow,
	type OrganizationRow
} from './crm-supabase-mappers';
import {
	contactResponseOf,
	opportunityResponseOf,
	organizationResponseOf,
	type CRMContactToolResult,
	type CRMOpportunityToolResult,
	type CRMOrganizationToolResult
} from './crm-tool-mappers';

const organizationSelection = 'id, name, status, types, tags, importance, owner_id, address, description, created_at, created_by, updated_at, updated_by, archived_at, archived_by';
const contactSelection = 'id, organization_id, name, email, phone, title, department, description, created_at, created_by, updated_at, updated_by, archived_at, archived_by';
const opportunitySelection = 'id, organization_id, contact_id, name, business, pipeline_id, stage_id, stage_position, stage_changed_at, owner_id, amount_minor, currency_code, base_amount_minor, base_currency_code, importance, due_at, due_time_zone, lost_reason, description, created_at, created_by, updated_at, updated_by, archived_at, archived_by';
const taskSelection = 'id, organization_id, opportunity_id, contact_id, title, note, business, type, due_at, starts_at, ends_at, is_event, is_whole_day, notify_minutes_before, location, created_at, updated_at, requester_id, status, task_participant (member_id)';

type CRMContext = { companyID: string; memberID: string };

export async function loadSupabaseCRMData(): Promise<CRMDataResponse> {
	const client = supabase();
	const [company, organizations, contacts, opportunities, tasks] = await Promise.all([
		client.from('company').select('crm_vocabulary, task_vocabulary').limit(1).single<{ crm_vocabulary: unknown; task_vocabulary: unknown }>(),
		client.from('organization').select(organizationSelection).order('name').returns<OrganizationRow[]>(),
		client.from('contact').select(contactSelection).order('name').returns<ContactRow[]>(),
		client.from('opportunity').select(opportunitySelection).order('stage_position').returns<OpportunityRow[]>(),
		client.from('task').select(taskSelection).not('organization_id', 'is', null).order('created_at', { ascending: false }).returns<CRMTaskRow[]>()
	]);
	throwResultError(company.error);
	throwResultError(organizations.error);
	throwResultError(contacts.error);
	throwResultError(opportunities.error);
	throwResultError(tasks.error);
	return crmDataResponseOf(
		organizations.data ?? [],
		contacts.data ?? [],
		opportunities.data ?? [],
		tasks.data ?? [],
		requiredData(company.data, 'company').crm_vocabulary,
		requiredData(company.data, 'company').task_vocabulary
	);
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
	const opened = await callTool<CRMOpportunityToolResult>('crm_opportunity_add', {
		...opportunityInput(payload),
		...(settlingStage(transition.stage) ? {} : { stage: transition.stage })
	});
	return opportunityResponseOf(opened);
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
	const context = await crmContext();
	const saved = await supabase().rpc('crm_task_save', crmTaskArguments(null, payload, context));
	throwResultError(saved.error);
	return activityFrom(await taskByID(requiredData(saved.data as string | null, 'task id')));
}

export async function updateSupabaseCRMActivity(id: string, payload: CRMActivityPayload): Promise<CRMActivityResponse> {
	const context = await crmContext();
	const saved = await supabase().rpc('crm_task_save', crmTaskArguments(id, payload, context));
	throwResultError(saved.error);
	return activityFrom(await taskByID(requiredData(saved.data as string | null, 'task id')));
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

async function crmContext(): Promise<CRMContext> {
	const client = supabase();
	const session = await client.auth.getSession();
	if (session.error) throwResultError(session.error);
	const accountID = session.data.session?.user.id;
	if (!accountID) throw new CRMApiError('sign in first', 401, 'unauthenticated');
	const member = await client
		.from('member')
		.select('id, company_id')
		.eq('user_id', accountID)
		.single<{ id: string; company_id: string }>();
	throwResultError(member.error);
	const row = requiredData(member.data, 'member');
	return { companyID: row.company_id, memberID: row.id };
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

function settlingStage(stage: string): boolean {
	return stage === 'done' || stage === 'lost';
}

function crmTaskArguments(taskID: string | null, payload: CRMActivityPayload, context: CRMContext) {
	const ownerID = payload.taskOwnerID || context.memberID;
	return {
		target_task_id: taskID,
		target_title: payload.title,
		target_status: payload.taskStatus || 'todo',
		target_note: payload.content || null,
		target_business: payload.business || null,
		target_type: payload.kind,
		target_due_at: payload.isEvent ? null : payload.occurredAt,
		target_starts_at: payload.isEvent ? payload.startsAt || payload.occurredAt : null,
		target_ends_at: payload.isEvent ? payload.endsAt || payload.startsAt || payload.occurredAt : null,
		target_is_event: payload.isEvent,
		target_is_whole_day: payload.isEvent && payload.isWholeDay,
		target_notify_minutes_before: payload.isEvent ? payload.notifyMinutesBefore : null,
		target_location: payload.isEvent && payload.location ? { name: payload.location } : null,
		target_requester_id: null,
		target_participant_ids: [ownerID],
		target_organization_id: payload.organizationID,
		target_opportunity_id: payload.opportunityID || null,
		target_contact_id: payload.contactID || null
	};
}

async function taskByID(id: string): Promise<CRMTaskRow> {
	const result = await supabase().from('task').select(taskSelection).eq('id', id).single<CRMTaskRow>();
	throwResultError(result.error);
	return requiredData(result.data, 'task');
}

function requiredTransition(value: CRMTransitionPayload | undefined): CRMTransitionPayload {
	if (value) return value;
	throw new CRMApiError('진행 단계가 필요합니다.', 400, 'stage_required');
}

function activityFrom(row: CRMTaskRow): CRMActivityResponse {
	return crmDataResponseOf([], [], [], [row], {}).activities[0];
}

function throwResultError(error: { message: string; code?: string } | null): void {
	if (!error) return;
	const status = error.code === '42501' ? 403 : error.code === '2BP01' ? 409 : 500;
	throw new CRMApiError(error.message, status, error.code ?? 'request_failed');
}

function requiredData<T>(value: T | null, label: string): T {
	if (value !== null) return value;
	throw new CRMApiError(`${label} response is empty`, 502, 'invalid_response');
}
