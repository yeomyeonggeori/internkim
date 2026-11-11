import { humanizedKey, type CompanyFieldNames } from './company';
import type { CompanyText } from './company-text';
import type { CompanyFieldKind } from './model';

export type FieldName = (kind: CompanyFieldKind, field: string) => string;

function productFieldName(kind: CompanyFieldKind, field: string, text: CompanyText): string | undefined {
	if (kind === 'profile') return text.fields[field];
	if (kind === 'metric') return text.metricNames[field];
	return text.recordCategories[field];
}

export function companyFieldName(
	kind: CompanyFieldKind,
	field: string,
	names: Record<string, string> | null | undefined,
	text: CompanyText,
	locale: string
): string {
	return names?.[locale] ?? productFieldName(kind, field, text) ?? humanizedKey(field);
}

export function fieldNamer(fieldNames: CompanyFieldNames, text: CompanyText, locale: string): FieldName {
	const namesOf = new Map(fieldNames.map((named) => [`${named.kind}:${named.field}`, named.names]));
	return (kind, field) => companyFieldName(kind, field, namesOf.get(`${kind}:${field}`), text, locale);
}
