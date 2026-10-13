import { z } from 'zod';
import { moneySchema, formatMoney } from '$lib/money';

const localizedTextSchema = z.record(z.string(), z.string());

export const dataRoomCompanySchema = z.object({
	profile: z.object({
		name: localizedTextSchema.optional(),
		brandName: localizedTextSchema.optional(),
		slogan: localizedTextSchema.optional(),
		description: localizedTextSchema.optional(),
		representative: localizedTextSchema.optional(),
		representativeTitle: localizedTextSchema.optional(),
		address: localizedTextSchema.optional(),
		officeAddress: localizedTextSchema.optional(),
		jurisdiction: localizedTextSchema.optional(),
		bankAccount: localizedTextSchema.optional(),
		legalAttributes: z.record(z.string(), z.record(z.string(), z.string())).optional(),
		foundedDate: z.string().optional(),
		capital: moneySchema.optional(),
		fiscalYearEnd: z.string().optional(),
		employeeCount: z.number().optional(),
		phone: z.string().optional(),
		fax: z.string().optional(),
		email: z.string().optional(),
		website: z.string().optional()
	}),
	metrics: z.array(z.object({
		metric: z.string(),
		year: z.number().int(),
		quarter: z.number().int().optional(),
		month: z.number().int().optional(),
		value: z.number(),
		currency: z.string().optional(),
		unit: z.string().optional(),
		note: z.string().optional()
	})),
	records: z.array(z.object({
		category: z.string(),
		date: z.string().optional(),
		title: z.string(),
		detail: z.string().optional(),
		attributes: z.record(z.string(), z.unknown())
	}))
});

export type DataRoomCompany = z.infer<typeof dataRoomCompanySchema>;
export type CompanyProfile = DataRoomCompany['profile'];
export type CompanyMetric = DataRoomCompany['metrics'][number];
export type CompanyRecord = DataRoomCompany['records'][number];
export type CompanyMetricSeries = { metric: string; latest: CompanyMetric; points: CompanyMetric[] };
export type CompanyFact = { field: string; value: string };

export function isEmptyCompany(company: DataRoomCompany): boolean {
	return Object.keys(company.profile).length === 0 && company.metrics.length === 0 && company.records.length === 0;
}

export function localized(text: Record<string, string> | undefined, locale: string): string {
	if (!text) return '';
	return text[locale] || text.en || Object.values(text).find(Boolean) || '';
}

export function companyFacts(profile: CompanyProfile, locale: string): CompanyFact[] {
	const representative = [localized(profile.representativeTitle, locale), localized(profile.representative, locale)]
		.filter(Boolean).join(' ');
	const facts: CompanyFact[] = [
		{ field: 'foundedDate', value: profile.foundedDate ?? '' },
		{ field: 'representative', value: representative },
		{ field: 'employeeCount', value: profile.employeeCount === undefined ? '' : new Intl.NumberFormat(locale).format(profile.employeeCount) },
		{ field: 'capital', value: profile.capital ? formatMoney(profile.capital, locale) : '' },
		{ field: 'jurisdiction', value: localized(profile.jurisdiction, locale) },
		{ field: 'fiscalYearEnd', value: profile.fiscalYearEnd ?? '' },
		{ field: 'address', value: localized(profile.address, locale) },
		{ field: 'officeAddress', value: localized(profile.officeAddress, locale) },
		{ field: 'phone', value: profile.phone ?? '' },
		{ field: 'bankAccount', value: localized(profile.bankAccount, locale) },
		...Object.entries(legalAttributesIn(profile, locale)).map(([label, value]) => ({ field: label, value }))
	];
	return facts.filter((fact) => fact.value);
}

function legalAttributesIn(profile: CompanyProfile, locale: string): Record<string, string> {
	const slots = profile.legalAttributes ?? {};
	return slots[locale] ?? slots.en ?? Object.values(slots).find((labels) => Object.keys(labels).length > 0) ?? {};
}

export function metricSeries(metrics: CompanyMetric[]): CompanyMetricSeries[] {
	const byName = Map.groupBy(metrics, (metric) => metric.metric);
	return [...byName].map(([metric, points]) => {
		const ordered = points.toSorted((left, right) => periodOrder(left) - periodOrder(right));
		return { metric, latest: ordered[ordered.length - 1], points: ordered };
	});
}

export function periodOrder(metric: CompanyMetric): number {
	return metric.year * 100 + (metric.month ?? (metric.quarter ? metric.quarter * 3 : 12));
}

export function periodLabel(metric: CompanyMetric, locale: string): string {
	if (metric.month) return `${metric.year}.${String(metric.month).padStart(2, '0')}`;
	if (metric.quarter) return locale === 'ko' ? `${metric.year}년 ${metric.quarter}분기` : `${metric.year} Q${metric.quarter}`;
	return String(metric.year);
}

export function changeFromPrevious(series: CompanyMetricSeries): number | undefined {
	const previous = series.points.at(-2);
	if (!previous || previous.value === 0) return undefined;
	return (series.latest.value - previous.value) / Math.abs(previous.value) * 100;
}

export function formatMetricValue(metric: CompanyMetric, locale: string, value = metric.value): string {
	if (metric.currency) {
		return new Intl.NumberFormat(locale, {
			style: 'currency', currency: metric.currency, currencyDisplay: 'narrowSymbol', notation: 'compact', maximumFractionDigits: 1
		}).format(value);
	}
	const number = new Intl.NumberFormat(locale, { notation: 'compact', maximumFractionDigits: 1 }).format(value);
	return metric.unit ? `${number} ${metric.unit}` : number;
}

export function humanizedKey(key: string): string {
	return key
		.replace(/[_-]+/g, ' ')
		.replace(/([a-z0-9])([A-Z])/g, '$1 $2')
		.replace(/^\w/, (character) => character.toUpperCase());
}

export function recordAttributes(record: CompanyRecord): [string, string][] {
	return Object.entries(record.attributes)
		.filter(([, value]) => value !== null && value !== '')
		.map(([key, value]) => [humanizedKey(key), typeof value === 'string' ? value : JSON.stringify(value)]);
}
