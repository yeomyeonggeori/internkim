import type { DataRoomCompany } from './company';

export type CompanyFieldKind = 'profile' | 'metric' | 'record' | 'empty';
export type CompanyFieldRow = { field: string; kind: CompanyFieldKind; categoryCode: string };
type CategoryFields = { code: string; parent: string | null; fields?: string[] };
type ReadingCircle = { id: string; readableCategories: string[] };

export const unlistedCategory = 'X';

export function companyFieldRows(categories: CategoryFields[], company: DataRoomCompany): CompanyFieldRow[] {
	const categoryOf = new Map(categories.flatMap((category) => (category.fields ?? []).map((field) => [field, category.code])));
	const present: [CompanyFieldKind, string[]][] = [
		['profile', [...Object.keys(company.profile), ...(company.activity ? ['activity'] : [])]],
		['metric', company.metrics.map((metric) => metric.metric)],
		['record', company.records.map((record) => record.category)]
	];
	const presentFields = new Set(present.flatMap(([, fields]) => fields));
	const empty: [CompanyFieldKind, string[]] = ['empty', [...categoryOf.keys()].filter((field) => !presentFields.has(field))];
	return [...present, empty].flatMap(([kind, fields]) =>
		[...new Set(fields)].map((field) => ({ field, kind, categoryCode: categoryOf.get(field) ?? unlistedCategory }))
	);
}

export function circlesReading<Circle extends ReadingCircle>(code: string, circles: Circle[], categories: CategoryFields[]): Circle[] {
	const parent = categories.find((category) => category.code === code)?.parent;
	return circles.filter((circle) =>
		circle.readableCategories.some((readable) => readable === code || readable === parent)
	);
}
