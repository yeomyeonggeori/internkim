import type { CRMRecordImportDuplicateChoice, CRMRecordImportKind } from '$lib/crm/crm-record-import';
import { crmImportances, crmOrganizationStatuses } from '$lib/crm/crm-record-values';
import type { CRMImportLine } from './crm-import-csv';

export const crmImportFields = {
	organization: ['name', 'types', 'status', 'importance', 'tags', 'ownerPersonHint', 'address', 'description'],
	contact: ['organizationName', 'name', 'email', 'phoneNumber', 'role', 'department', 'description']
} as const;

export const crmImportFieldsOfEveryKind = [
	...new Set([...crmImportFields.organization, ...crmImportFields.contact])
];

export type CRMImportField =
	| (typeof crmImportFields.organization)[number]
	| (typeof crmImportFields.contact)[number];

export const crmImportRequiredFields: Record<CRMRecordImportKind, readonly CRMImportField[]> = {
	organization: ['name'],
	contact: ['organizationName', 'name']
};

const listFields: readonly CRMImportField[] = ['types', 'tags'];
const listSeparator = /[,;]/;

export type CRMImportMapping = (CRMImportField | null)[];

export type CRMImportRow = Partial<Record<CRMImportField, string | string[]>> & {
	line: number;
	name: string;
	onDuplicate?: CRMRecordImportDuplicateChoice;
};

type CRMImportLabelSet = Readonly<Record<string, string>>;

export type CRMImportValueLabels = {
	status: readonly CRMImportLabelSet[];
	importance: readonly CRMImportLabelSet[];
};

function headerKey(header: string): string {
	return header.trim().toLowerCase();
}

export function suggestedMapping(
	headers: string[],
	fields: readonly CRMImportField[],
	namesOf: (field: CRMImportField) => string[]
): CRMImportMapping {
	const taken = new Set<CRMImportField>();
	return headers.map((header) => {
		const field = fields.find(
			(candidate) =>
				!taken.has(candidate) && namesOf(candidate).some((name) => headerKey(name) === headerKey(header))
		);
		if (!field) return null;
		taken.add(field);
		return field;
	});
}

export function withField(mapping: CRMImportMapping, column: number, field: CRMImportField | null): CRMImportMapping {
	return mapping.map((held, index) => {
		if (index === column) return field;
		return field !== null && held === field ? null : held;
	});
}

export function missingRequiredFields(kind: CRMRecordImportKind, mapping: CRMImportMapping): CRMImportField[] {
	return crmImportRequiredFields[kind].filter((field) => !mapping.includes(field));
}

function keyOfLabel(value: string, keys: readonly string[], labelSets: readonly CRMImportLabelSet[]): string {
	const trimmed = value.trim();
	return keys.find((key) => key === trimmed || labelSets.some((labels) => labels[key] === trimmed)) ?? trimmed;
}

function fieldValue(field: CRMImportField, cell: string, labels: CRMImportValueLabels): string | string[] {
	if (listFields.includes(field)) return cell.split(listSeparator).map((item) => item.trim()).filter(Boolean);
	if (field === 'status') return keyOfLabel(cell, crmOrganizationStatuses, labels.status);
	if (field === 'importance') return keyOfLabel(cell, crmImportances, labels.importance);
	return cell;
}

export function importedRows(
	mapping: CRMImportMapping,
	lines: CRMImportLine[],
	labels: CRMImportValueLabels
): CRMImportRow[] {
	return lines.map(({ line, cells }) => {
		const values: Partial<Record<CRMImportField, string | string[]>> = {};
		mapping.forEach((field, column) => {
			if (field) values[field] = fieldValue(field, cells[column] ?? '', labels);
		});
		return { ...values, line, name: typeof values.name === 'string' ? values.name : '' };
	});
}
