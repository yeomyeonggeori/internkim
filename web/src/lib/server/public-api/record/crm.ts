import type { SupabaseClient } from '@supabase/supabase-js';
import { crmStageKeys, crmStageOutcomes, type CRMStage } from '$lib/crm/crm-stage';
import { emailNearness, titleNearness, typoNearness } from './hint-nearness';
import { HintRefused, normalized, resolveHint, type HintCandidate, type HintMatcher } from './hint-resolution';
import type { CRMVocabularyResult } from '../catalog/crm';

type CRMAuditColumns = {
	created_at: string;
	created_by: string | null;
	updated_at: string;
	updated_by: string | null;
	archived_at: string | null;
	archived_by: string | null;
};

export type OrganizationRow = CRMAuditColumns & {
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

export type ContactRow = CRMAuditColumns & {
	id: string;
	organization_id: string | null;
	name: string;
	email: string | null;
	phone: string | null;
	title: string | null;
	department: string | null;
	description: string | null;
};

export type OpportunityRow = CRMAuditColumns & {
	id: string;
	organization_id: string;
	contact_id: string | null;
	name: string;
	business: string | null;
	pipeline_id: string;
	stage_id: CRMStage;
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
	lost_reason: string | null;
	description: string | null;
};

export type CRMCompanyRow = {
	id: string;
	currency_code: string;
	crm_vocabulary: unknown;
};

export type CRMDefinition = { id: string; name: string; color?: string };
export type CRMPipelineDefinition = CRMDefinition & { direction?: string };
export type CRMVocabulary = { organization_types: CRMDefinition[]; pipelines: CRMPipelineDefinition[] };

export const organizationColumns =
	'id, name, status, types, tags, importance, owner_id, address, description, created_at, created_by, updated_at, updated_by, archived_at, archived_by';
export const contactColumns =
	'id, organization_id, name, email, phone, title, department, description, created_at, created_by, updated_at, updated_by, archived_at, archived_by';
export const opportunityColumns =
	'id, organization_id, contact_id, name, business, pipeline_id, stage_id, stage_position, stage_changed_at, owner_id, amount_minor, currency_code, base_amount_minor, base_currency_code, importance, due_at, due_time_zone, lost_reason, description, created_at, created_by, updated_at, updated_by, archived_at, archived_by';

export async function crmCompanyRow(caller: SupabaseClient): Promise<CRMCompanyRow> {
	const { data, error } = await caller
		.from('company')
		.select('id, currency_code, crm_vocabulary')
		.limit(1)
		.single<CRMCompanyRow>();
	if (error) throw new Error(error.message);
	return data;
}

export async function organizationsOfCompany(
	caller: SupabaseClient,
	includeArchived = false
): Promise<OrganizationRow[]> {
	return crmRows<OrganizationRow>(caller, 'organization', organizationColumns, 'name', includeArchived);
}

export async function contactsOfCompany(
	caller: SupabaseClient,
	includeArchived = false
): Promise<ContactRow[]> {
	return crmRows<ContactRow>(caller, 'contact', contactColumns, 'name', includeArchived);
}

export async function opportunitiesOfCompany(
	caller: SupabaseClient,
	includeArchived = false
): Promise<OpportunityRow[]> {
	return crmRows<OpportunityRow>(caller, 'opportunity', opportunityColumns, 'stage_position', includeArchived);
}

export async function activityCountByOpportunity(caller: SupabaseClient): Promise<Map<string, number>> {
	const counted = new Map<string, number>();
	for (let from = 0; ; from += rowsPerPage) {
		const { data, error } = await caller.from('task').select('id, opportunity_id')
			.not('opportunity_id', 'is', null).order('id').range(from, from + rowsPerPage - 1)
			.returns<{ id: string; opportunity_id: string }[]>();
		if (error) throw new Error(error.message);
		const rows = data ?? [];
		for (const row of rows) counted.set(row.opportunity_id, (counted.get(row.opportunity_id) ?? 0) + 1);
		if (rows.length < rowsPerPage) return counted;
	}
}

const rowsPerPage = 500;

async function crmRows<Row>(caller: SupabaseClient, table: string, columns: string, order: string, includeArchived: boolean): Promise<Row[]> {
	const rows: Row[] = [];
	for (let from = 0; ; from += rowsPerPage) {
		let query = caller.from(table).select(columns);
		if (!includeArchived) query = query.is('archived_at', null);
		const { data, error } = await query.order(order).order('id')
			.range(from, from + rowsPerPage - 1).returns<Row[]>();
		if (error) throw new Error(error.message);
		const page = data ?? [];
		rows.push(...page);
		if (page.length < rowsPerPage) return rows;
	}
}

const organizationMatcher: HintMatcher<OrganizationRow> = {
	identifiersOf: (organization) => [organization.id],
	titleOf: (organization) => organization.name,
	nearnessTo: (organization, hint) =>
		Math.max(
			typoNearness(normalized(hint), normalized(organization.name)),
			titleNearness(normalized(hint), normalized(organization.name))
		)
};

const contactMatcher: HintMatcher<ContactRow> = {
	identifiersOf: (contact) => [contact.id, contact.email ?? '', localPartOf(contact.email)],
	titleOf: (contact) => contact.name,
	nearnessTo: (contact, hint) =>
		Math.max(
			typoNearness(normalized(hint), normalized(contact.name)),
			emailNearness(normalized(hint), normalized(contact.email ?? ''))
		)
};

const opportunityMatcher: HintMatcher<OpportunityRow> = {
	identifiersOf: (opportunity) => [opportunity.id],
	titleOf: (opportunity) => opportunity.name,
	nearnessTo: (opportunity, hint) => titleNearness(normalized(hint), normalized(opportunity.name))
};

function localPartOf(email: string | null): string {
	const at = (email ?? '').indexOf('@');
	return at > 0 ? (email ?? '').slice(0, at) : '';
}

export function organizationOfHint(rows: OrganizationRow[], hint: string): OrganizationRow {
	const resolution = resolveHint(hint, rows, organizationMatcher);
	if (resolution.outcome === 'resolved') return resolution.match;
	throw new HintRefused(
		'crmOrganization',
		hint.trim(),
		resolution.outcome,
		resolution.candidates.map((organization) => ({ id: organization.id, label: organization.name }))
	);
}

export function contactOfHint(rows: ContactRow[], hint: string): ContactRow {
	const resolution = resolveHint(hint, rows, contactMatcher);
	if (resolution.outcome === 'resolved') return resolution.match;
	throw new HintRefused(
		'crmContact',
		hint.trim(),
		resolution.outcome,
		resolution.candidates.map(candidateOfContact)
	);
}

function candidateOfContact(contact: ContactRow): HintCandidate {
	return {
		id: contact.id,
		label: contact.name,
		...(contact.email ? { email: contact.email } : {})
	};
}

export function opportunityOfHint(rows: OpportunityRow[], hint: string): OpportunityRow {
	const resolution = resolveHint(hint, rows, opportunityMatcher);
	if (resolution.outcome === 'resolved') return resolution.match;
	throw new HintRefused(
		'crmOpportunity',
		hint.trim(),
		resolution.outcome,
		resolution.candidates.map((opportunity) => ({ id: opportunity.id, label: opportunity.name }))
	);
}

export function vocabularyOfCompany(stored: unknown): CRMVocabulary {
	if (typeof stored !== 'object' || stored === null) return { organization_types: [], pipelines: [] };
	const held = stored as Record<string, unknown>;
	return {
		organization_types: definitionsOf(held.organization_types),
		pipelines: definitionsOf(held.pipelines)
	};
}

function definitionsOf(value: unknown): CRMPipelineDefinition[] {
	if (!Array.isArray(value)) return [];
	return value.filter(isDefinition).map((definition) => ({
		id: definition.id,
		name: definition.name,
		...(typeof definition.color === 'string' ? { color: definition.color } : {}),
		...(typeof definition.direction === 'string' ? { direction: definition.direction } : {})
	}));
}

function isDefinition(
	value: unknown
): value is { id: string; name: string; color?: unknown; direction?: unknown } {
	if (typeof value !== 'object' || value === null) return false;
	const definition = value as Record<string, unknown>;
	return typeof definition.id === 'string' && typeof definition.name === 'string';
}

export function answeredVocabulary(vocabulary: CRMVocabulary): CRMVocabularyResult {
	return {
		organizationTypes: vocabulary.organization_types,
		pipelines: vocabulary.pipelines,
		stages: crmStageKeys.map((stage) => ({ stage, outcome: crmStageOutcomes[stage] }))
	};
}
