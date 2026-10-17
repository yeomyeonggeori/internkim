import { z } from 'zod';
import { formatCompactMoney, formatMoney, moneySchema, type Money } from '$lib/money';
import { companyActivitySchema, storedActivitySchema } from './activity';

const localizedTextSchema = z.record(z.string(), z.string());

export const holderKinds = ['founder', 'employee', 'investor', 'option_pool', 'other'] as const;
const holderKindSchema = z.enum(holderKinds);
const heldByKindSchema = z.array(z.object({ kind: holderKindSchema, shares: z.number() }));

const recordSchema = z.object({
	category: z.string(),
	date: z.string().optional(),
	title: z.string(),
	detail: z.string().optional(),
	status: z.enum(['in_progress', 'completed']).optional(),
	amount: moneySchema.optional(),
	committed: moneySchema.optional(),
	attributes: z.record(z.string(), z.unknown())
});

const ownershipSchema = z.object({
	asOf: z.string(),
	kinds: heldByKindSchema,
	rounds: z.array(z.object({ title: z.string(), date: z.string().optional(), kinds: heldByKindSchema }))
});

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
		amount: moneySchema.optional(),
		usdEquivalent: z.number().optional(),
		value: z.number().optional(),
		unit: z.string().optional(),
		note: z.string().optional()
	})),
	records: z.array(recordSchema),
	activity: companyActivitySchema.optional(),
	ownership: ownershipSchema.optional()
});

export const storedCompanySchema = dataRoomCompanySchema.extend({
	records: z.array(recordSchema.omit({ committed: true }).extend({
		committedMinor: z.object({ amountMinor: z.number(), currency: z.string() }).optional()
	})),
	activity: storedActivitySchema.optional()
});

export type DataRoomCompany = z.infer<typeof dataRoomCompanySchema>;
export type StoredCompany = z.infer<typeof storedCompanySchema>;
export type CompanyProfile = DataRoomCompany['profile'];
export type CompanyMetric = DataRoomCompany['metrics'][number];
export type CompanyRecord = DataRoomCompany['records'][number];
export type CompanyOwnership = NonNullable<DataRoomCompany['ownership']>;
export type HolderKind = (typeof holderKinds)[number];
export type CompanyMetricSeries = { metric: string; latest: CompanyMetric; points: CompanyMetric[] };
export type CompanyFact = { field: string; value: string };
export type MetricChange = { kind: 'percent'; percent: number } | { kind: 'turnedPositive' } | { kind: 'turnedNegative' };
export type PeriodKind = 'year' | 'quarter' | 'month';
export type RecordGroup = { category: string; records: CompanyRecord[] };

export const fundingCategories = ['funding', 'grant'];
export const proofCategories = ['certification', 'ip', 'award', 'reference'];
const headlineMetric = 'annualRevenue';
export const headcountMetric = 'employees';

export function isEmptyCompany(company: DataRoomCompany): boolean {
	return Object.keys(company.profile).length === 0 && company.metrics.length === 0 && company.records.length === 0
		&& !company.activity && !company.ownership;
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
		{ field: 'capital', value: profile.capital ? formatMoney(profile.capital, locale) : '' },
		{ field: 'jurisdiction', value: localized(profile.jurisdiction, locale) },
		{ field: 'fiscalYearEnd', value: fiscalYearEndLabel(profile.fiscalYearEnd, locale) },
		{ field: 'address', value: localized(profile.address, locale) },
		{ field: 'officeAddress', value: localized(profile.officeAddress, locale) },
		{ field: 'phone', value: profile.phone ?? '' },
		{ field: 'bankAccount', value: localized(profile.bankAccount, locale) },
		...Object.entries(legalAttributesIn(profile, locale)).map(([label, value]) => ({ field: label, value }))
	];
	return facts.filter((fact) => fact.value);
}

export function fiscalYearEndLabel(fiscalYearEnd: string | undefined, locale: string): string {
	if (!fiscalYearEnd) return '';
	const month = Number(fiscalYearEnd);
	if (!Number.isInteger(month) || month < 1 || month > 12) return fiscalYearEnd;
	return new Intl.DateTimeFormat(locale, { month: 'long', timeZone: 'UTC' }).format(Date.UTC(2000, month - 1, 1));
}

function legalAttributesIn(profile: CompanyProfile, locale: string): Record<string, string> {
	const slots = profile.legalAttributes ?? {};
	return slots[locale] ?? slots.en ?? Object.values(slots).find((labels) => Object.keys(labels).length > 0) ?? {};
}

export function metricSeries(metrics: CompanyMetric[]): CompanyMetricSeries[] {
	const byName = Map.groupBy(metrics, (metric) => metric.metric);
	return [...byName].map(([metric, points]) => {
		const ordered = points.toSorted((left, right) => periodOrder(left) - periodOrder(right));
		const latest = ordered[ordered.length - 1];
		return { metric, latest, points: ordered.filter((point) => periodKind(point) === periodKind(latest)) };
	});
}

export function periodKind(metric: CompanyMetric): PeriodKind {
	if (metric.month) return 'month';
	if (metric.quarter) return 'quarter';
	return 'year';
}

export function periodOrder(metric: CompanyMetric): number {
	return metric.year * 100 + (metric.month ?? (metric.quarter ? metric.quarter * 3 : 12));
}

export function periodLabel(metric: CompanyMetric, locale: string): string {
	if (metric.month) return `${metric.year}.${String(metric.month).padStart(2, '0')}`;
	if (metric.quarter) return locale === 'ko' ? `${metric.year}년 ${metric.quarter}분기` : `${metric.year} Q${metric.quarter}`;
	return String(metric.year);
}

export type MetricBar = { period: string; value: number; share: number; isLatest: boolean };
export type MetricBarChart = { bars: MetricBar[]; positiveShare: number; negativeShare: number };

export function metricBars(series: CompanyMetricSeries, periodCount: number, locale: string): MetricBarChart {
	const points = series.points.slice(-periodCount);
	const values = points.map(metricNumber);
	const highest = Math.max(0, ...values);
	const lowest = Math.min(0, ...values);
	const range = highest - lowest;
	const bars = points.map((point, index) => ({
		period: shortPeriodLabel(point, locale),
		value: values[index],
		share: values[index] > 0 ? values[index] / highest : values[index] < 0 ? values[index] / lowest : 0,
		isLatest: index === points.length - 1
	}));
	if (range === 0) return { bars, positiveShare: 1, negativeShare: 0 };
	return { bars, positiveShare: highest / range, negativeShare: -lowest / range };
}

export function shortPeriodLabel(metric: CompanyMetric, locale: string): string {
	const shortYear = String(metric.year).slice(-2);
	if (metric.month) return `${shortYear}.${String(metric.month).padStart(2, '0')}`;
	if (metric.quarter) return locale === 'ko' ? `${shortYear}년 ${metric.quarter}분기` : `Q${metric.quarter} '${shortYear}`;
	return String(metric.year);
}

export function metricNumber(metric: CompanyMetric): number {
	return metric.amount?.amount ?? metric.value ?? 0;
}

export function changeFromPrevious(series: CompanyMetricSeries): MetricChange | undefined {
	const previous = series.points.at(-2);
	if (!previous || metricNumber(previous) === 0) return undefined;
	const [before, after] = [metricNumber(previous), metricNumber(series.latest)];
	if (before < 0 && after >= 0) return { kind: 'turnedPositive' };
	if (before > 0 && after < 0) return { kind: 'turnedNegative' };
	return { kind: 'percent', percent: (after - before) / Math.abs(before) * 100 };
}

export function headlineSeries(series: CompanyMetricSeries[]): CompanyMetricSeries | undefined {
	return series.find((candidate) => candidate.metric === headlineMetric)
		?? series.find((candidate) => candidate.latest.amount !== undefined);
}

export function headcount(company: DataRoomCompany): number | undefined {
	if (company.profile.employeeCount !== undefined) return company.profile.employeeCount;
	const series = metricSeries(company.metrics).find((candidate) => candidate.metric === headcountMetric);
	return series ? metricNumber(series.latest) : undefined;
}

export function yearsOperating(foundedDate: string | undefined, today: Date): number | undefined {
	const foundedYear = Number(foundedDate?.slice(0, 4));
	if (!foundedYear) return undefined;
	return today.getFullYear() - foundedYear + 1;
}

export function recordsIn(records: CompanyRecord[], categories: string[]): CompanyRecord[] {
	return records.filter((record) => categories.includes(record.category));
}

export function isInProgress(record: CompanyRecord): boolean {
	return record.status === 'in_progress';
}

export function completedRecords(records: CompanyRecord[]): CompanyRecord[] {
	return records.filter((record) => !isInProgress(record));
}

export function roundBeingRaised(records: CompanyRecord[]): CompanyRecord | undefined {
	return recordsIn(records, ['funding']).filter(isInProgress).toSorted(newestFirst)[0];
}

export type OwnershipShare = { kind: HolderKind; shares: number; percent: number };

export function ownershipShares(held: CompanyOwnership['kinds']): OwnershipShare[] {
	const total = held.reduce((sum, entry) => sum + entry.shares, 0);
	return holderKinds.flatMap((kind) => {
		const shares = held.find((entry) => entry.kind === kind)?.shares ?? 0;
		return shares ? [{ kind, shares, percent: total ? (shares / total) * 100 : 0 }] : [];
	});
}

export function totalShares(held: CompanyOwnership['kinds']): number {
	return held.reduce((sum, entry) => sum + entry.shares, 0);
}

export function totalRaised(records: CompanyRecord[]): Money | undefined {
	const amounts = completedRecords(recordsIn(records, ['funding'])).flatMap((record) => (record.amount ? [record.amount] : []));
	const currency = amounts[0]?.currency;
	if (!currency || amounts.some((amount) => amount.currency !== currency)) return undefined;
	return { amount: amounts.reduce((sum, amount) => sum + amount.amount, 0), currency };
}

export function proofGroups(records: CompanyRecord[]): RecordGroup[] {
	return proofCategories
		.map((category) => ({ category, records: recordsIn(records, [category]) }))
		.filter((group) => group.records.length > 0);
}

export function milestoneRecords(records: CompanyRecord[]): CompanyRecord[] {
	const grouped = new Set([...fundingCategories, ...proofCategories]);
	return records.filter((record) => !grouped.has(record.category)).toSorted(newestFirst);
}

export function newestFirst(left: CompanyRecord, right: CompanyRecord): number {
	if (!left.date || !right.date) return Number(!left.date) - Number(!right.date);
	return right.date.localeCompare(left.date);
}

export function formatMetricValue(metric: CompanyMetric, locale: string, number = metricNumber(metric)): string {
	if (metric.amount) return formatCompactMoney({ amount: number, currency: metric.amount.currency }, locale);
	const formatted = new Intl.NumberFormat(locale, { notation: 'compact', maximumSignificantDigits: 3 }).format(number);
	if (!metric.unit) return formatted;
	return /^[A-Za-z%]/.test(metric.unit) ? `${formatted} ${metric.unit}` : `${formatted}${metric.unit}`;
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
		.map(([key, value]) => [key, typeof value === 'string' ? value : JSON.stringify(value)]);
}
