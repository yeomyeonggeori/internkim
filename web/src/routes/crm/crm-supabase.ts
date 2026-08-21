import { supabase } from '$lib/supabase';
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
	const context = await crmContext();
	const result = await supabase()
		.from('organization')
		.insert({ company_id: context.companyID, ...organizationValues(payload) })
		.select(organizationSelection)
		.single<OrganizationRow>();
	throwResultError(result.error);
	return organizationFrom(requiredData(result.data, 'organization'));
}

export async function updateSupabaseCRMOrganization(id: string, payload: CRMOrganizationPayload): Promise<CRMOrganizationResponse> {
	const result = await supabase()
		.from('organization')
		.update(organizationValues(payload))
		.eq('id', id)
		.select(organizationSelection)
		.single<OrganizationRow>();
	throwResultError(result.error);
	return organizationFrom(requiredData(result.data, 'organization'));
}

export async function archiveSupabaseCRMOrganization(id: string): Promise<void> {
	await archiveRecord('organization', id);
}

export async function createSupabaseCRMContact(payload: CRMContactPayload): Promise<CRMContactResponse> {
	const context = await crmContext();
	const result = await supabase()
		.from('contact')
		.insert({ company_id: context.companyID, messenger: {}, ...contactValues(payload) })
		.select(contactSelection)
		.single<ContactRow>();
	throwResultError(result.error);
	return contactFrom(requiredData(result.data, 'contact'));
}

export async function updateSupabaseCRMContact(id: string, payload: CRMContactPayload): Promise<CRMContactResponse> {
	const result = await supabase()
		.from('contact')
		.update(contactValues(payload))
		.eq('id', id)
		.select(contactSelection)
		.single<ContactRow>();
	throwResultError(result.error);
	return contactFrom(requiredData(result.data, 'contact'));
}

export async function archiveSupabaseCRMContact(id: string): Promise<void> {
	await archiveRecord('contact', id);
}

export async function createSupabaseCRMOpportunity(payload: CRMOpportunityPayload): Promise<CRMOpportunityResponse> {
	const context = await crmContext();
	const transition = requiredTransition(payload.transition);
	const result = await supabase()
		.from('opportunity')
		.insert({
			company_id: context.companyID,
			...opportunityValues(payload),
			stage_id: transition.stage,
			stage_position: transition.stagePosition,
			stage_changed_at: transition.occurredAt,
			lost_reason: transition.lostReason || null,
			base_amount_minor: transition.baseAmountMinor,
			base_currency_code: transition.baseCurrencyCode || null
		})
		.select(opportunitySelection)
		.single<OpportunityRow>();
	throwResultError(result.error);
	return opportunityFrom(requiredData(result.data, 'opportunity'));
}

export async function updateSupabaseCRMOpportunity(id: string, payload: CRMOpportunityPayload): Promise<CRMOpportunityResponse> {
	const transitionValues = payload.transition ? {
		stage_id: payload.transition.stage,
		stage_position: payload.transition.stagePosition,
		stage_changed_at: payload.transition.occurredAt,
		lost_reason: payload.transition.lostReason || null,
		base_amount_minor: payload.transition.baseAmountMinor,
		base_currency_code: payload.transition.baseCurrencyCode || null
	} : {};
	const result = await supabase()
		.from('opportunity')
		.update({ ...opportunityValues(payload), ...transitionValues })
		.eq('id', id)
		.select(opportunitySelection)
		.single<OpportunityRow>();
	throwResultError(result.error);
	return opportunityFrom(requiredData(result.data, 'opportunity'));
}

export async function archiveSupabaseCRMOpportunity(id: string): Promise<void> {
	await archiveRecord('opportunity', id);
}

export async function transitionSupabaseCRMOpportunity(id: string, payload: CRMTransitionPayload): Promise<CRMOpportunityResponse> {
	const session = await supabase().auth.getSession();
	const accessToken = session.data.session?.access_token;
	if (!accessToken) throw new CRMApiError('sign in first', 401, 'unauthenticated');

	const response = await fetch('/api/crm/opportunity-close', {
		method: 'POST',
		headers: { Authorization: `Bearer ${accessToken}`, 'Content-Type': 'application/json' },
		body: JSON.stringify({
			opportunityID: id,
			stage: payload.stage,
			stagePosition: payload.stagePosition,
			occurredAt: payload.occurredAt,
			lostReason: payload.lostReason
		})
	});
	if (!response.ok) {
		const detail = (await response.text()).trim();
		throw new CRMApiError(detail || `closing returned ${response.status}`, response.status, 'close_failed');
	}
	const settled = (await response.json()) as { opportunity: OpportunityRow };
	return opportunityFrom(requiredData(settled.opportunity, 'opportunity'));
}

export async function positionSupabaseCRMOpportunity(id: string, payload: CRMPositionPayload): Promise<CRMOpportunityResponse> {
	const result = await supabase()
		.from('opportunity')
		.update({ stage_position: payload.position })
		.eq('id', id)
		.select(opportunitySelection)
		.single<OpportunityRow>();
	throwResultError(result.error);
	return opportunityFrom(requiredData(result.data, 'opportunity'));
}

export async function createSupabaseCRMActivity(payload: CRMActivityPayload): Promise<CRMActivityResponse> {
	const context = await crmContext();
	const saved = await supabase().rpc('save_crm_task', crmTaskArguments(null, payload, context));
	throwResultError(saved.error);
	return activityFrom(await taskByID(requiredData(saved.data as string | null, 'task id')));
}

export async function updateSupabaseCRMActivity(id: string, payload: CRMActivityPayload): Promise<CRMActivityResponse> {
	const context = await crmContext();
	const saved = await supabase().rpc('save_crm_task', crmTaskArguments(id, payload, context));
	throwResultError(saved.error);
	return activityFrom(await taskByID(requiredData(saved.data as string | null, 'task id')));
}

export async function saveSupabaseCRMVocabulary(vocabulary: CRMVocabulary): Promise<void> {
	const result = await supabase().rpc('save_crm_vocabulary', { target_vocabulary: vocabulary });
	throwResultError(result.error);
}

async function crmContext(): Promise<CRMContext> {
	const client = supabase();
	const session = await client.auth.getSession();
	if (session.error) throwResultError(session.error);
	const userID = session.data.session?.user.id;
	if (!userID) throw new CRMApiError('sign in first', 401, 'unauthenticated');
	const member = await client
		.from('member')
		.select('id, company_id')
		.eq('user_id', userID)
		.single<{ id: string; company_id: string }>();
	throwResultError(member.error);
	const row = requiredData(member.data, 'member');
	return { companyID: row.company_id, memberID: row.id };
}

function organizationValues(payload: CRMOrganizationPayload) {
	return {
		name: payload.name,
		status: payload.status,
		types: payload.types,
		tags: payload.tags,
		importance: payload.importance,
		owner_id: payload.ownerPersonID || null,
		address: payload.address || null,
		description: payload.description || null
	};
}

function contactValues(payload: CRMContactPayload) {
	return {
		organization_id: payload.organizationID || null,
		name: payload.name,
		email: payload.email || null,
		phone: payload.phone || null,
		title: payload.title || null,
		department: payload.department || null,
		description: payload.description || null
	};
}

function opportunityValues(payload: CRMOpportunityPayload) {
	return {
		organization_id: payload.organizationID,
		contact_id: payload.contacts[0]?.contactID || null,
		name: payload.name,
		business: payload.business || null,
		pipeline_id: payload.pipeline,
		owner_id: payload.ownerPersonID || null,
		amount_minor: payload.amountMinor,
		currency_code: payload.currencyCode || null,
		importance: payload.importance,
		due_at: payload.dueAt || null,
		due_time_zone: payload.dueTimeZone || null,
		description: payload.description || null
	};
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

async function archiveRecord(table: 'organization' | 'contact' | 'opportunity', id: string): Promise<void> {
	const result = await supabase().from(table).update({ archived_at: new Date().toISOString() }).eq('id', id);
	throwResultError(result.error);
}

function organizationFrom(row: OrganizationRow): CRMOrganizationResponse {
	return crmDataResponseOf([row], [], [], [], {}).organizations[0];
}

function contactFrom(row: ContactRow): CRMContactResponse {
	return crmDataResponseOf([], [row], [], [], {}).contacts[0];
}

function opportunityFrom(row: OpportunityRow): CRMOpportunityResponse {
	return crmDataResponseOf([], [], [row], [], {}).opportunities[0];
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
