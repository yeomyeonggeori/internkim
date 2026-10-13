import type { CRMRecordImportProblem } from '$lib/crm/crm-record-import';
import { crmImportances, crmOrganizationStatuses } from '$lib/crm/crm-record-values';
import {
	contactsOfCompany,
	crmCompanyRow,
	organizationsOfCompany,
	vocabularyOfCompany,
	type ContactRow,
	type CRMDefinition,
	type OrganizationRow
} from './crm';
import { HintRefused } from './hint-resolution';
import { personOfHint } from './people';
import { RecordRefusedTheWrite, statusOfPostgresCode } from './tasks';
import type { RecordContext } from './company';
import type {
	CRMRecordImportInput,
	CRMRecordImportResult,
	CRMRecordImportRow
} from '../catalog/crm';

type ImportedRowResult = CRMRecordImportResult['rows'][number];
type Change = Record<string, unknown>;

type CheckedRow = {
	result: ImportedRowResult;
	change: Change;
};

type HeldRecord = { id: string; change: Change };

const organizationOnlyFields = ['types', 'status', 'importance', 'tags', 'ownerPersonHint', 'address'] as const;
const contactOnlyFields = ['organizationName', 'email', 'phoneNumber', 'role', 'department'] as const;
const emailShape = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

function nameKey(name: string): string {
	return name.trim().toLowerCase();
}

function filled(value: string | undefined): string {
	return value?.trim() ?? '';
}

function filledList(values: string[] | undefined): string[] {
	return (values ?? []).map((value) => value.trim()).filter(Boolean);
}

function carriesAny(row: CRMRecordImportRow, fields: readonly (keyof CRMRecordImportRow)[]): boolean {
	return fields.some((field) => {
		const value = row[field];
		if (Array.isArray(value)) return filledList(value).length > 0;
		return typeof value === 'string' && value.trim() !== '';
	});
}

function withFilled(change: Change, column: string, value: string): void {
	if (value) change[column] = value;
}

function typeKeys(types: string[], vocabulary: CRMDefinition[]): string[] | null {
	const keys: string[] = [];
	for (const type of types) {
		const definition = vocabulary.find((held) => held.id === type || held.name.trim() === type);
		if (!definition) return null;
		keys.push(definition.id);
	}
	return keys;
}

function ownerID(context: RecordContext, hint: string): string | null {
	try {
		return personOfHint(context.people, hint).personID;
	} catch (refusal) {
		if (refusal instanceof HintRefused) return null;
		throw refusal;
	}
}

function organizationChange(
	context: RecordContext,
	row: CRMRecordImportRow,
	vocabulary: CRMDefinition[],
	problems: CRMRecordImportProblem[]
): Change {
	const change: Change = { name: row.name.trim() };
	const types = filledList(row.types);
	if (types.length > 0) {
		const keys = typeKeys(types, vocabulary);
		if (keys) change.types = keys;
		else problems.push('type_unknown');
	}
	const status = filled(row.status);
	if (status && !crmOrganizationStatuses.some((known) => known === status)) problems.push('status_unknown');
	withFilled(change, 'status', status);
	const importance = filled(row.importance);
	if (importance && !crmImportances.some((known) => known === importance)) problems.push('importance_unknown');
	withFilled(change, 'importance', importance);
	const tags = filledList(row.tags);
	if (tags.length > 0) change.tags = tags;
	const ownerHint = filled(row.ownerPersonHint);
	if (ownerHint) {
		const owner = ownerID(context, ownerHint);
		if (owner) change.owner_id = owner;
		else problems.push('owner_unresolved');
	}
	withFilled(change, 'address', filled(row.address));
	withFilled(change, 'description', filled(row.description));
	return change;
}

function contactChange(
	row: CRMRecordImportRow,
	organizationsByName: Map<string, OrganizationRow[]>,
	problems: CRMRecordImportProblem[]
): Change {
	const change: Change = { name: row.name.trim() };
	const organizations = organizationsByName.get(nameKey(row.organizationName ?? '')) ?? [];
	if (organizations.length === 1) change.organization_id = organizations[0].id;
	else problems.push(organizations.length === 0 ? 'organization_unknown' : 'organization_ambiguous');
	const email = filled(row.email);
	if (email && !emailShape.test(email)) problems.push('email_invalid');
	withFilled(change, 'email', email);
	withFilled(change, 'phone', filled(row.phoneNumber));
	withFilled(change, 'title', filled(row.role));
	withFilled(change, 'department', filled(row.department));
	withFilled(change, 'description', filled(row.description));
	return change;
}

function heldOrganization(row: OrganizationRow): HeldRecord {
	return {
		id: row.id,
		change: {
			name: row.name,
			status: row.status,
			types: row.types,
			tags: row.tags,
			importance: row.importance,
			owner_id: row.owner_id,
			address: row.address,
			description: row.description
		}
	};
}

function heldContact(row: ContactRow): HeldRecord {
	return {
		id: row.id,
		change: {
			name: row.name,
			organization_id: row.organization_id,
			email: row.email,
			phone: row.phone,
			title: row.title,
			department: row.department,
			description: row.description
		}
	};
}

function isReachable(change: Change): boolean {
	return Boolean(change.email || change.phone);
}

function groupedBy<Row>(rows: Row[], keyOf: (row: Row) => string): Map<string, Row[]> {
	const grouped = new Map<string, Row[]>();
	for (const row of rows) grouped.set(keyOf(row), [...(grouped.get(keyOf(row)) ?? []), row]);
	return grouped;
}

function contactKey(organizationID: unknown, name: string): string {
	return `${String(organizationID ?? '')}\u0000${nameKey(name)}`;
}

function checkedRows(
	input: CRMRecordImportInput,
	changeOf: (row: CRMRecordImportRow, problems: CRMRecordImportProblem[]) => Change,
	keyOf: (row: CRMRecordImportRow, change: Change) => string,
	heldByKey: Map<string, HeldRecord[]>,
	otherKindFields: readonly (keyof CRMRecordImportRow)[]
): CheckedRow[] {
	const linesByKey = new Map<string, number>();
	return input.rows.map((row) => {
		const problems: CRMRecordImportProblem[] = [];
		if (!row.name.trim()) problems.push('name_missing');
		if (carriesAny(row, otherKindFields)) problems.push('field_of_other_kind');
		const change = changeOf(row, problems);
		const key = keyOf(row, change);
		if (row.name.trim() && linesByKey.has(key)) problems.push('name_repeated_in_file');
		linesByKey.set(key, row.line);
		const duplicate = heldByKey.get(key)?.[0];
		const merged = duplicate && row.onDuplicate === 'update' ? { ...duplicate.change, ...change } : change;
		if (input.kind === 'contact' && !(duplicate && row.onDuplicate !== 'update') && !isReachable(merged)) {
			problems.push('contact_unreachable');
		}
		return {
			result: {
				line: row.line,
				outcome: problems.length > 0 ? 'refused' : duplicate ? (row.onDuplicate === 'update' ? 'update' : 'skip') : 'create',
				duplicateOfID: duplicate?.id ?? '',
				recordID: duplicate && row.onDuplicate === 'update' ? duplicate.id : '',
				problems
			},
			change: merged
		};
	});
}

async function checkedOrganizations(context: RecordContext, input: CRMRecordImportInput): Promise<CheckedRow[]> {
	const [organizations, company] = await Promise.all([
		organizationsOfCompany(context.caller),
		crmCompanyRow(context.caller)
	]);
	const vocabulary = vocabularyOfCompany(company.crm_vocabulary).organization_types;
	const heldByKey = groupedBy(organizations.map(heldOrganization), (held) => nameKey(String(held.change.name)));
	return checkedRows(
		input,
		(row, problems) => organizationChange(context, row, vocabulary, problems),
		(row) => nameKey(row.name),
		heldByKey,
		contactOnlyFields
	);
}

async function checkedContacts(context: RecordContext, input: CRMRecordImportInput): Promise<CheckedRow[]> {
	const [organizations, contacts] = await Promise.all([
		organizationsOfCompany(context.caller),
		contactsOfCompany(context.caller)
	]);
	const organizationsByName = groupedBy(organizations, (organization) => nameKey(organization.name));
	const heldByKey = groupedBy(contacts.map(heldContact), (held) =>
		contactKey(held.change.organization_id, String(held.change.name))
	);
	return checkedRows(
		input,
		(row, problems) => contactChange(row, organizationsByName, problems),
		(row, change) => contactKey(change.organization_id, row.name),
		heldByKey,
		organizationOnlyFields
	);
}

function withNewRecordIDs(checked: CheckedRow[]): CheckedRow[] {
	return checked.map((row) =>
		row.result.outcome === 'create' ? { ...row, result: { ...row.result, recordID: crypto.randomUUID() } } : row
	);
}

function writtenRows(context: RecordContext, checked: CheckedRow[]): Change[] {
	return checked
		.filter((row) => row.result.outcome !== 'skip')
		.map((row) => ({ id: row.result.recordID, company_id: context.companyID, ...row.change }));
}

async function writeAll(context: RecordContext, table: string, rows: Change[]): Promise<void> {
	if (rows.length === 0) return;
	const { error } = await context.caller
		.from(table)
		.upsert(rows, { onConflict: 'id', defaultToNull: false });
	if (error) throw new RecordRefusedTheWrite(error.message, statusOfPostgresCode(error.code));
}

function counted(checked: CheckedRow[], outcome: ImportedRowResult['outcome']): number {
	return checked.filter((row) => row.result.outcome === outcome).length;
}

export async function crmRecordImport(
	context: RecordContext,
	input: CRMRecordImportInput
): Promise<CRMRecordImportResult> {
	const checked =
		input.kind === 'organization'
			? await checkedOrganizations(context, input)
			: await checkedContacts(context, input);
	const dryRun = input.dryRun === true;
	const refused = counted(checked, 'refused');
	const written = !dryRun && refused === 0;
	const answered = written ? withNewRecordIDs(checked) : checked;
	if (written) await writeAll(context, input.kind, writtenRows(context, answered));
	return {
		dryRun,
		written,
		created: counted(checked, 'create'),
		updated: counted(checked, 'update'),
		skipped: counted(checked, 'skip'),
		refused,
		rows: answered.map((row) => row.result)
	};
}
