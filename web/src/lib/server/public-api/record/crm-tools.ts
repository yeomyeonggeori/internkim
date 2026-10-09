import type { SupabaseClient } from '@supabase/supabase-js';
import { crmStageSettles, isCRMStage, type CRMStage } from '$lib/crm/crm-stage';
import { convertedAmount } from '$lib/server/converted-amount';
import { frankfurterProvider } from '$lib/server/exchange-rates';
import {
	activityCountByOpportunity,
	answeredVocabulary,
	contactOfHint,
	contactsOfCompany,
	crmCompanyRow,
	contactColumns,
	opportunityColumns,
	opportunityOfHint,
	opportunitiesOfCompany,
	organizationColumns,
	organizationOfHint,
	organizationsOfCompany,
	vocabularyOfCompany,
	type ContactRow,
	type CRMPipelineDefinition,
	type CRMVocabulary,
	type OpportunityRow,
	type OrganizationRow
} from './crm';
import { instantWritten } from './days';
import { LabelUnresolved } from './labels';
import { personOfHint } from './people';
import { RecordRefusedTheWrite, statusOfPostgresCode } from './tasks';
import type { RecordContext } from './company';
import type {
	CRMArchivedResult,
	CRMContactListResult,
	CRMContactResult,
	CRMOpportunityListResult,
	CRMOpportunityResult,
	CRMOrganizationListResult,
	CRMOrganizationResult,
	CRMVocabularyResult
} from '../catalog/crm';

export type CRMColumnsWritten = {
	organizationHint?: string;
	opportunityHint?: string;
	contactHint?: string;
	dueAt?: string;
};

export type CRMColumnHolder = {
	organization_id: string | null;
	opportunity_id: string | null;
	contact_id: string | null;
};

export type CRMOrganizationListInput = { query?: string; includeArchived?: boolean };

export type CRMOrganizationWritten = {
	name?: string;
	types?: string[];
	status?: string;
	importance?: string;
	tags?: string[];
	ownerPersonHint?: string;
	address?: string;
	description?: string;
};

export type CRMContactListInput = {
	organizationHint?: string;
	query?: string;
	includeArchived?: boolean;
};

export type CRMContactWritten = {
	name?: string;
	organizationHint?: string;
	email?: string;
	phoneNumber?: string;
	role?: string;
	department?: string;
	description?: string;
};

export type CRMOpportunityListInput = {
	organizationHint?: string;
	stage?: string;
	query?: string;
	includeArchived?: boolean;
};

export type CRMOpportunityWritten = {
	title?: string;
	organizationHint?: string;
	pipeline?: string;
	business?: string;
	amountMinor?: number;
	currencyCode?: string;
	contactHint?: string;
	ownerPersonHint?: string;
	importance?: string;
	expectedCloseDate?: string;
	description?: string;
	stage?: string;
};

export type CRMOpportunityMoveInput = {
	opportunityHint?: string;
	stage?: string;
	position?: number;
	closedAt?: string;
	finalAmountMinor?: number;
	reason?: string;
};

export type CRMVocabularySetInput = {
	organizationTypes?: CRMPipelineDefinition[];
	pipelines?: CRMPipelineDefinition[];
};

const exchangeRates = frankfurterProvider();

function searchableText(value: string): string {
	return value.trim().toLowerCase().split(/\s+/).join('');
}

function matchesQuery(searched: (string | null)[], query: string | undefined): boolean {
	const asked = searchableText(query ?? '');
	if (!asked) return true;
	return searched.some((value) => searchableText(value ?? '').includes(asked));
}

function hintOf(offered: string | undefined, subject: string): string {
	const hint = offered?.trim();
	if (!hint) throw new Error(`this call names the ${subject} it is about`);
	return hint;
}

function auditOf(row: {
	created_at: string;
	created_by: string | null;
	updated_at: string;
	updated_by: string | null;
	archived_at: string | null;
	archived_by: string | null;
}) {
	return {
		createdAt: row.created_at,
		createdByPersonID: row.created_by ?? '',
		updatedAt: row.updated_at,
		updatedByPersonID: row.updated_by ?? '',
		archivedAt: row.archived_at,
		archivedByPersonID: row.archived_by ?? ''
	};
}

function answeredOrganization(row: OrganizationRow): CRMOrganizationResult {
	return {
		organizationID: row.id,
		name: row.name,
		status: row.status,
		types: row.types ?? [],
		tags: row.tags ?? [],
		importance: row.importance,
		ownerPersonID: row.owner_id ?? '',
		address: row.address ?? '',
		description: row.description ?? '',
		audit: auditOf(row)
	};
}

function answeredContact(row: ContactRow): CRMContactResult {
	return {
		contactID: row.id,
		organizationID: row.organization_id ?? '',
		name: row.name,
		email: row.email ?? '',
		phoneNumber: row.phone ?? '',
		role: row.title ?? '',
		department: row.department ?? '',
		description: row.description ?? '',
		audit: auditOf(row)
	};
}

function answeredOpportunity(row: OpportunityRow, activityCount: number): CRMOpportunityResult {
	return {
		opportunityID: row.id,
		organizationID: row.organization_id,
		contactID: row.contact_id ?? '',
		title: row.name,
		business: row.business ?? '',
		pipeline: row.pipeline_id,
		stage: row.stage_id,
		stagePosition: row.stage_position,
		stageChangedAt: row.stage_changed_at,
		ownerPersonID: row.owner_id ?? '',
		amountMinor: row.amount_minor,
		currencyCode: row.currency_code ?? '',
		baseAmountMinor: row.base_amount_minor,
		baseCurrencyCode: row.base_currency_code ?? '',
		importance: row.importance,
		expectedCloseAt: row.due_at ?? '',
		expectedCloseTimeZone: row.due_time_zone ?? '',
		lostReason: row.lost_reason ?? '',
		description: row.description ?? '',
		activityCount,
		audit: auditOf(row)
	};
}

function archivedResult(id: string, name: string, archivedAt: string | null): CRMArchivedResult {
	return { recordID: id, name, archivedAt: archivedAt ?? '' };
}

async function writtenRow<Row>(
	caller: SupabaseClient,
	table: string,
	columns: string,
	change: Record<string, unknown>,
	id: string | null
): Promise<Row> {
	const written = id
		? caller.from(table).update(change).eq('id', id)
		: caller.from(table).insert(change);
	const { data, error } = await written.select(columns).returns<Row[]>();
	if (error) throw new RecordRefusedTheWrite(error.message, statusOfPostgresCode(error.code));
	const rows = data ?? [];
	if (rows.length === 1) return rows[0];
	throw new RecordRefusedTheWrite(
		`only somebody who works here can change this company's ${table} records`,
		403
	);
}

export async function organizationOfCRMHint(
	context: RecordContext,
	hint: string
): Promise<OrganizationRow> {
	return organizationOfHint(await organizationsOfCompany(context.caller), hint);
}

export async function contactOfCRMHint(context: RecordContext, hint: string): Promise<ContactRow> {
	return contactOfHint(await contactsOfCompany(context.caller), hint);
}

export async function opportunityOfCRMHint(
	context: RecordContext,
	hint: string
): Promise<OpportunityRow> {
	return opportunityOfHint(await opportunitiesOfCompany(context.caller), hint);
}

function ownerOf(context: RecordContext, hint: string | undefined, held: string | null): string | null {
	if (hint === undefined) return held;
	if (!hint.trim()) return null;
	return personOfHint(context.people, hint).personID;
}

function trimmedOr(written: string | undefined, held: string | null): string | null {
	if (written === undefined) return held;
	return written.trim() || null;
}

function organizationChange(
	context: RecordContext,
	input: CRMOrganizationWritten,
	row: OrganizationRow | null
): Record<string, unknown> {
	const change: Record<string, unknown> = {};
	if (input.name !== undefined) change.name = input.name.trim();
	if (input.status !== undefined) change.status = input.status.trim();
	if (input.importance !== undefined) change.importance = input.importance.trim();
	if (input.types !== undefined) change.types = input.types.map((type) => type.trim()).filter(Boolean);
	if (input.tags !== undefined) change.tags = input.tags.map((tag) => tag.trim()).filter(Boolean);
	if (input.address !== undefined) change.address = trimmedOr(input.address, null);
	if (input.description !== undefined) change.description = trimmedOr(input.description, null);
	if (input.ownerPersonHint !== undefined) {
		change.owner_id = ownerOf(context, input.ownerPersonHint, row?.owner_id ?? null);
	}
	return change;
}

export async function crmOrganizationList(
	context: RecordContext,
	input: CRMOrganizationListInput
): Promise<CRMOrganizationListResult> {
	const rows = (await organizationsOfCompany(context.caller, input.includeArchived === true)).filter(
		(row) => matchesQuery([row.name, row.address, row.description], input.query)
	);
	return { count: rows.length, organizations: rows.map(answeredOrganization) };
}

export async function crmOrganizationAdd(
	context: RecordContext,
	input: CRMOrganizationWritten
): Promise<CRMOrganizationResult> {
	if (!input.name?.trim()) throw new Error('an organization needs a name');
	const written = await writtenRow<OrganizationRow>(
		context.caller,
		'organization',
		organizationColumns,
		{ company_id: context.companyID, ...organizationChange(context, input, null) },
		null
	);
	return answeredOrganization(written);
}

export async function crmOrganizationUpdate(
	context: RecordContext,
	input: CRMOrganizationWritten & { organizationHint?: string }
): Promise<CRMOrganizationResult> {
	const row = await organizationOfCRMHint(context, hintOf(input.organizationHint, 'organization'));
	const change = organizationChange(context, input, row);
	if (Object.keys(change).length === 0) throw new Error('an update names at least one field to change');

	return answeredOrganization(
		await writtenRow<OrganizationRow>(context.caller, 'organization', organizationColumns, change, row.id)
	);
}

export async function crmOrganizationArchive(
	context: RecordContext,
	input: { organizationHint?: string }
): Promise<CRMArchivedResult> {
	const row = await organizationOfCRMHint(context, hintOf(input.organizationHint, 'organization'));
	const written = await writtenRow<OrganizationRow>(
		context.caller,
		'organization',
		organizationColumns,
		{ archived_at: context.now.toISOString() },
		row.id
	);
	return archivedResult(written.id, written.name, written.archived_at);
}

async function contactChange(
	context: RecordContext,
	input: CRMContactWritten,
	row: ContactRow | null
): Promise<Record<string, unknown>> {
	const change: Record<string, unknown> = {};
	if (input.name !== undefined) change.name = input.name.trim();
	if (input.email !== undefined) change.email = trimmedOr(input.email, null);
	if (input.phoneNumber !== undefined) change.phone = trimmedOr(input.phoneNumber, null);
	if (input.role !== undefined) change.title = trimmedOr(input.role, null);
	if (input.department !== undefined) change.department = trimmedOr(input.department, null);
	if (input.description !== undefined) change.description = trimmedOr(input.description, null);
	if (input.organizationHint !== undefined) {
		const organization = await organizationOfCRMHint(context, hintOf(input.organizationHint, 'organization'));
		change.organization_id = organization.id;
	} else if (!row) {
		throw new Error('a contact names the organization they work at');
	}
	return change;
}

export async function crmContactList(
	context: RecordContext,
	input: CRMContactListInput
): Promise<CRMContactListResult> {
	const organizationID = input.organizationHint
		? (await organizationOfCRMHint(context, input.organizationHint)).id
		: '';
	const rows = (await contactsOfCompany(context.caller, input.includeArchived === true)).filter(
		(row) =>
			(!organizationID || row.organization_id === organizationID) &&
			matchesQuery([row.name, row.email, row.title], input.query)
	);
	return { count: rows.length, contacts: rows.map(answeredContact) };
}

function refuseAContactNobodyCanReach(change: Record<string, unknown>, row: ContactRow | null): void {
	const written = (field: 'email' | 'phone'): string => {
		const value = field in change ? change[field] : row?.[field];
		return typeof value === 'string' ? value.trim() : '';
	};
	if (written('email') === '' && written('phone') === '') {
		throw new Error('a contact needs an email address or a phone number');
	}
}

export async function crmContactAdd(
	context: RecordContext,
	input: CRMContactWritten
): Promise<CRMContactResult> {
	if (!input.name?.trim()) throw new Error('a contact needs a name');
	const change = await contactChange(context, input, null);
	refuseAContactNobodyCanReach(change, null);
	const written = await writtenRow<ContactRow>(
		context.caller,
		'contact',
		contactColumns,
		{ company_id: context.companyID, messenger: {}, ...change },
		null
	);
	return answeredContact(written);
}

export async function crmContactUpdate(
	context: RecordContext,
	input: CRMContactWritten & { contactHint?: string }
): Promise<CRMContactResult> {
	const row = await contactOfCRMHint(context, hintOf(input.contactHint, 'contact'));
	const change = await contactChange(context, input, row);
	if (Object.keys(change).length === 0) throw new Error('an update names at least one field to change');
	refuseAContactNobodyCanReach(change, row);

	return answeredContact(await writtenRow<ContactRow>(context.caller, 'contact', contactColumns, change, row.id));
}

export async function crmContactArchive(
	context: RecordContext,
	input: { contactHint?: string }
): Promise<CRMArchivedResult> {
	const row = await contactOfCRMHint(context, hintOf(input.contactHint, 'contact'));
	const written = await writtenRow<ContactRow>(
		context.caller,
		'contact',
		contactColumns,
		{ archived_at: context.now.toISOString() },
		row.id
	);
	return archivedResult(written.id, written.name, written.archived_at);
}

// A pipeline a company never registered would open a column its board does not
// draw, so an unregistered one is refused with the list rather than written.
function pipelineOf(vocabulary: CRMVocabulary, asked: string | undefined, held: string | null): string {
	const registered = vocabulary.pipelines;
	const written = asked?.trim() ?? '';
	if (!written) {
		if (held) return held;
		if (registered.length === 1) return registered[0].id;
		if (registered.length === 0) {
			throw new Error('this company registered no pipeline to open a deal in; crm_vocabulary_set names them');
		}
		throw new Error(
			`this company runs deals through ${registered.map((pipeline) => pipeline.name).join(', ')}; name which one`
		);
	}

	const exact = registered.find(
		(pipeline) =>
			pipeline.id.toLowerCase() === written.toLowerCase() ||
			pipeline.name.toLowerCase() === written.toLowerCase()
	);
	if (exact) return exact.id;
	if (registered.length === 0) return written;

	const contained = registered.filter((pipeline) =>
		pipeline.name.toLowerCase().includes(written.toLowerCase())
	);
	if (contained.length === 1) return contained[0].id;
	throw new LabelUnresolved(
		written,
		registered.map((pipeline) => pipeline.name),
		contained.length > 1 ? 'ambiguous' : 'unregistered'
	);
}

function stageWritten(asked: string | undefined): CRMStage | undefined {
	const written = asked?.trim();
	if (!written) return undefined;
	if (!isCRMStage(written)) throw new Error(`${written} is not a stage a deal can stand at`);
	return written;
}

async function opportunityChange(
	context: RecordContext,
	vocabulary: CRMVocabulary,
	input: CRMOpportunityWritten,
	row: OpportunityRow | null
): Promise<Record<string, unknown>> {
	const change: Record<string, unknown> = {};
	if (input.title !== undefined) change.name = input.title.trim();
	if (input.business !== undefined) change.business = trimmedOr(input.business, null);
	if (input.importance !== undefined) change.importance = input.importance.trim();
	if (input.description !== undefined) change.description = trimmedOr(input.description, null);
	if (input.amountMinor !== undefined) change.amount_minor = input.amountMinor;
	if (input.currencyCode !== undefined) {
		change.currency_code = input.currencyCode.trim().toUpperCase() || null;
	}
	if (input.ownerPersonHint !== undefined) {
		change.owner_id = ownerOf(context, input.ownerPersonHint, row?.owner_id ?? null);
	}
	if (input.expectedCloseDate !== undefined) {
		const written = input.expectedCloseDate.trim();
		change.due_at = written ? instantWritten(context.labels.timezone, written, true) : null;
		change.due_time_zone = written ? context.labels.timezone : null;
	}
	if (input.pipeline !== undefined || !row) {
		change.pipeline_id = pipelineOf(vocabulary, input.pipeline, row?.pipeline_id ?? null);
	}
	if (input.organizationHint !== undefined || !row) {
		const organization = await organizationOfCRMHint(
			context,
			hintOf(input.organizationHint, 'organization')
		);
		change.organization_id = organization.id;
	}
	if (input.contactHint !== undefined) {
		const written = input.contactHint.trim();
		change.contact_id = written ? (await contactOfCRMHint(context, written)).id : null;
	}
	return change;
}

export async function crmOpportunityList(
	context: RecordContext,
	input: CRMOpportunityListInput
): Promise<CRMOpportunityListResult> {
	const organizationID = input.organizationHint
		? (await organizationOfCRMHint(context, input.organizationHint)).id
		: '';
	const stage = stageWritten(input.stage);
	const [activityCounts, opportunities] = await Promise.all([
		activityCountByOpportunity(context.caller),
		opportunitiesOfCompany(context.caller, input.includeArchived === true)
	]);
	const rows = opportunities.filter(
		(row) =>
			(!organizationID || row.organization_id === organizationID) &&
			(!stage || row.stage_id === stage) &&
			matchesQuery([row.name, row.description], input.query)
	);
	return {
		count: rows.length,
		opportunities: rows.map((row) => answeredOpportunity(row, activityCounts.get(row.id) ?? 0))
	};
}

export async function crmOpportunityAdd(
	context: RecordContext,
	input: CRMOpportunityWritten
): Promise<CRMOpportunityResult> {
	if (!input.title?.trim()) throw new Error('a deal needs a title');
	const stage = stageWritten(input.stage) ?? 'waiting';
	if (crmStageSettles(stage)) {
		throw new Error('a deal is opened at an open stage and closed with crm_opportunity_move');
	}

	const vocabulary = vocabularyOfCompany((await crmCompanyRow(context.caller)).crm_vocabulary);
	const written = await writtenRow<OpportunityRow>(
		context.caller,
		'opportunity',
		opportunityColumns,
		{
			company_id: context.companyID,
			...(await opportunityChange(context, vocabulary, input, null)),
			stage_id: stage,
			stage_position: 0,
			stage_changed_at: context.now.toISOString()
		},
		null
	);
	return answeredOpportunity(written, 0);
}

export async function crmOpportunityUpdate(
	context: RecordContext,
	input: CRMOpportunityWritten & { opportunityHint?: string }
): Promise<CRMOpportunityResult> {
	const row = await opportunityOfCRMHint(context, hintOf(input.opportunityHint, 'deal'));
	const vocabulary = vocabularyOfCompany((await crmCompanyRow(context.caller)).crm_vocabulary);
	const change = await opportunityChange(context, vocabulary, input, row);
	if (Object.keys(change).length === 0) throw new Error('an update names at least one field to change');

	const { owner_id: ownerID, ...edit } = change;
	const edited = Object.keys(edit).length > 0
		? await writtenRow<OpportunityRow>(context.caller, 'opportunity', opportunityColumns, edit, row.id)
		: row;
	const written = ownerID === undefined || ownerID === row.owner_id
		? edited
		: await handedOverOpportunity(context, row.id, ownerID);
	return answeredOpportunity(written, (await activityCountByOpportunity(context.caller)).get(row.id) ?? 0);
}

async function handedOverOpportunity(
	context: RecordContext,
	opportunityID: string,
	ownerID: unknown
): Promise<OpportunityRow> {
	const { data, error } = await context.caller
		.rpc('crm_opportunity_hand_over', { target_opportunity_id: opportunityID, target_owner_id: ownerID })
		.single<OpportunityRow>();
	if (error) throw new RecordRefusedTheWrite(error.message, statusOfPostgresCode(error.code));
	return data;
}

export async function crmOpportunityMove(
	context: RecordContext,
	input: CRMOpportunityMoveInput
): Promise<CRMOpportunityResult> {
	const row = await opportunityOfCRMHint(context, hintOf(input.opportunityHint, 'deal'));
	const stage = stageWritten(input.stage);
	if (!stage) return reorderedOpportunity(context, row, input.position);

	const company = await crmCompanyRow(context.caller);
	const settled = crmStageSettles(stage)
		? await settledAmountOf(row, company.currency_code, input.finalAmountMinor)
		: null;

	const { data, error } = await context.caller.rpc('crm_opportunity_close', {
		target_opportunity_id: row.id,
		target_stage_id: stage,
		target_stage_position: input.position ?? row.stage_position,
		target_stage_changed_at: input.closedAt
			? instantWritten(context.labels.timezone, input.closedAt)
			: context.now.toISOString(),
		target_lost_reason: input.reason?.trim() || null,
		target_base_amount_minor: settled?.amountMinor ?? null,
		target_base_currency_code: settled?.currencyCode ?? null
	});
	if (error) throw new RecordRefusedTheWrite(error.message, statusOfPostgresCode(error.code));
	if (!data) throw new RecordRefusedTheWrite('the record named no deal it moved', 502);

	const moved = await opportunityOfCRMHint(context, row.id);
	return answeredOpportunity(moved, (await activityCountByOpportunity(context.caller)).get(row.id) ?? 0);
}

async function reorderedOpportunity(
	context: RecordContext,
	row: OpportunityRow,
	position: number | undefined
): Promise<CRMOpportunityResult> {
	if (position === undefined) throw new Error('a move names the stage or the position it moves the deal to');
	const written = await writtenRow<OpportunityRow>(
		context.caller,
		'opportunity',
		opportunityColumns,
		{ stage_position: position },
		row.id
	);
	return answeredOpportunity(written, (await activityCountByOpportunity(context.caller)).get(row.id) ?? 0);
}

async function settledAmountOf(
	row: OpportunityRow,
	companyCurrency: string,
	finalAmountMinor: number | undefined
): Promise<{ amountMinor: number; currencyCode: string } | null> {
	if (finalAmountMinor !== undefined) {
		return { amountMinor: finalAmountMinor, currencyCode: companyCurrency };
	}
	if (row.base_amount_minor !== null) return null;
	if (row.amount_minor === null || row.currency_code === null) return null;
	if (row.currency_code === companyCurrency) return null;

	return convertedAmount(exchangeRates, row.amount_minor, row.currency_code, companyCurrency);
}

export async function crmOpportunityArchive(
	context: RecordContext,
	input: { opportunityHint?: string }
): Promise<CRMArchivedResult> {
	const row = await opportunityOfCRMHint(context, hintOf(input.opportunityHint, 'deal'));
	const written = await writtenRow<OpportunityRow>(
		context.caller,
		'opportunity',
		opportunityColumns,
		{ archived_at: context.now.toISOString() },
		row.id
	);
	return archivedResult(written.id, written.name, written.archived_at);
}

export async function crmVocabularyGet(context: RecordContext): Promise<CRMVocabularyResult> {
	return answeredVocabulary(vocabularyOfCompany((await crmCompanyRow(context.caller)).crm_vocabulary));
}

export async function crmVocabularySet(
	context: RecordContext,
	input: CRMVocabularySetInput
): Promise<CRMVocabularyResult> {
	const { error } = await context.caller.rpc('crm_vocabulary_save', {
		target_vocabulary: {
			organization_types: (input.organizationTypes ?? []).map(definitionWritten),
			pipelines: (input.pipelines ?? []).map(definitionWritten)
		}
	});
	if (error) throw refusedVocabularyWrite(error.message, error.code);
	return crmVocabularyGet(context);
}

const dependentObjectsStillExist = '2BP01';

function refusedVocabularyWrite(reason: string, code: string | undefined): RecordRefusedTheWrite {
	if (code === dependentObjectsStillExist) {
		return new RecordRefusedTheWrite(reason, 409, 'crm_definition_in_use');
	}
	return new RecordRefusedTheWrite(reason, statusOfPostgresCode(code));
}

function definitionWritten(definition: CRMPipelineDefinition): Record<string, unknown> {
	return {
		id: definition.id.trim(),
		name: definition.name.trim(),
		...(definition.color?.trim() ? { color: definition.color.trim() } : {}),
		...(definition.direction?.trim() ? { direction: definition.direction.trim() } : {})
	};
}

// task_crm_references_match_organization refuses a task whose deal, contact and
// organization disagree, so the columns task_save does not carry are resolved
// against each other and written in one go.
export async function crmColumnsWritten(
	context: RecordContext,
	input: CRMColumnsWritten,
	row: CRMColumnHolder | null
): Promise<Record<string, unknown> | null> {
	const namesNothing =
		input.organizationHint === undefined &&
		input.opportunityHint === undefined &&
		input.contactHint === undefined;
	if (namesNothing && input.dueAt === undefined) return null;

	const dueAt =
		input.dueAt === undefined
			? {}
			: { due_at: input.dueAt.trim() ? instantWritten(context.labels.timezone, input.dueAt) : null };
	if (namesNothing) return dueAt;

	return { ...(await linksWritten(context, input, row)), ...dueAt };
}

async function linksWritten(
	context: RecordContext,
	input: CRMColumnsWritten,
	row: CRMColumnHolder | null
): Promise<Record<string, unknown>> {
	if (input.organizationHint !== undefined && !input.organizationHint.trim()) {
		return { organization_id: null, opportunity_id: null, contact_id: null };
	}

	const dealHint =
		input.opportunityHint === undefined ? row?.opportunity_id ?? '' : input.opportunityHint.trim();
	const deal = dealHint ? await opportunityOfCRMHint(context, dealHint) : null;
	const named = input.organizationHint
		? await organizationOfCRMHint(context, input.organizationHint)
		: null;
	if (deal && named && deal.organization_id !== named.id) {
		throw new Error(`${deal.name} is a deal with another organization`);
	}

	const contactHint = input.contactHint === undefined ? row?.contact_id ?? '' : input.contactHint.trim();
	const contact = contactHint ? await contactOfCRMHint(context, contactHint) : null;
	const organizationID = deal?.organization_id ?? named?.id ?? row?.organization_id ?? null;
	return {
		organization_id: organizationID,
		opportunity_id: deal?.id ?? null,
		contact_id: contact?.id ?? null
	};
}

export async function writeCRMColumns(
	context: RecordContext,
	taskID: string,
	columns: Record<string, unknown> | null
): Promise<void> {
	if (!columns) return;
	const { data, error } = await context.caller
		.from('task')
		.update(columns)
		.eq('id', taskID)
		.select('id')
		.returns<{ id: string }[]>();
	if (error) throw new RecordRefusedTheWrite(error.message, statusOfPostgresCode(error.code));
	if ((data ?? []).length === 0) {
		throw new RecordRefusedTheWrite(
			'only somebody taking part in this task or an admin can put it on an organization',
			403,
			'task_write_forbidden'
		);
	}
}
