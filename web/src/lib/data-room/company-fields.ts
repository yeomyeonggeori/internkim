import type { DataRoomCompany } from './company';
import type { CompanyFieldKind } from './model';

export type CompanyFieldGroup = CompanyFieldKind | 'empty';
export type CompanyFieldRow = {
	kind: CompanyFieldKind;
	field: string;
	group: CompanyFieldGroup;
	categoryCode: string;
	names: Record<string, string> | null;
};
type PlacedField = { kind: CompanyFieldKind; field: string; categoryCode: string | null; names: Record<string, string> | null };
type CategoryParent = { code: string; parent: string | null };
type ReadingCircle = { id: string; readableCategories: string[] };

export const unlistedCategory = 'X';

export function companyFieldIdentity(row: { kind: CompanyFieldKind; field: string }): string {
	return `${row.kind}:${row.field}`;
}

function rowOf(kind: CompanyFieldKind, field: string, group: CompanyFieldGroup, placed: PlacedField | undefined): CompanyFieldRow {
	return { kind, field, group, categoryCode: placed?.categoryCode ?? unlistedCategory, names: placed?.names ?? null };
}

function presentFields(company: DataRoomCompany): [CompanyFieldKind, string[]][] {
	return [
		['profile', [...Object.keys(company.profile), ...(company.activity ? ['activity'] : []), ...(company.ownership ? ['ownership'] : [])]],
		['metric', company.metrics.map((metric) => metric.metric)],
		['record', company.records.map((record) => record.category)]
	];
}

export function companyFieldRows(placed: PlacedField[], company: DataRoomCompany): CompanyFieldRow[] {
	const placedOf = new Map(placed.map((entry) => [companyFieldIdentity(entry), entry]));
	const present = presentFields(company).flatMap(([kind, fields]) => [...new Set(fields)].map((field) => ({ kind, field })));
	const presentIdentities = new Set(present.map(companyFieldIdentity));
	const presentRows = present.map(({ kind, field }) => rowOf(kind, field, kind, placedOf.get(companyFieldIdentity({ kind, field }))));
	const emptyRows = placed
		.filter((entry) => entry.categoryCode !== null && !presentIdentities.has(companyFieldIdentity(entry)))
		.map((entry) => rowOf(entry.kind, entry.field, 'empty', entry));
	return [...presentRows, ...emptyRows];
}

export function circlesReading<Circle extends ReadingCircle>(code: string, circles: Circle[], categories: CategoryParent[]): Circle[] {
	const parent = categories.find((category) => category.code === code)?.parent;
	return circles.filter((circle) =>
		circle.readableCategories.some((readable) => readable === code || readable === parent)
	);
}
