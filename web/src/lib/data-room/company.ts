import { z } from 'zod';
import { personName } from '$lib/person-name';
import { compareMomentsNewestFirst, momentLabel, momentSchema, momentSpanOf, type Moment, type MomentPrecision, type MomentSpan } from '$lib/moment';
import { formatCompactMoney, formatMoney, moneySchema, type Money } from '$lib/money';
import { companyActivitySchema, storedActivitySchema } from './activity';

const localizedTextSchema = z.record(z.string(), z.string());

export const holderKinds = ['founder', 'employee', 'investor', 'option_pool', 'other'] as const;
const holderKindSchema = z.enum(holderKinds);
const heldByKindSchema = z.array(z.object({ kind: holderKindSchema, shares: z.number() }));

const recordSchema = z.object({
	category: z.string(),
	date: momentSchema.optional(),
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
	rounds: z.array(z.object({ title: z.string(), date: momentSchema.optional(), kinds: heldByKindSchema }))
});

export const dataRoomCompanySchema = z.object({
	profile: z.object({
		name: localizedTextSchema.optional(),
		brandName: localizedTextSchema.optional(),
		slogan: localizedTextSchema.optional(),
		description: localizedTextSchema.optional(),
		representative: z.string().optional(),
		representativeTitle: z.string().optional(),
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
		period: momentSchema,
		amount: moneySchema.optional(),
		usdEquivalent: z.number().optional(),
		value: z.number().optional(),
		unit: z.string().optional(),
		note: z.string().optional(),
		recordedAt: z.string().optional()
	})),
	records: z.array(recordSchema),
	activity: companyActivitySchema.optional(),
	ownership: ownershipSchema.optional(),
	logo: z.string().optional()
});

export const storedCompanySchema = dataRoomCompanySchema.extend({
	records: z.array(recordSchema.omit({ committed: true }).extend({
		committedMinor: z.object({ amountMinor: z.number(), currency: z.string() }).optional()
	})),
	activity: storedActivitySchema.optional(),
	logoPath: z.string().optional()
});

export type DataRoomCompany = z.infer<typeof dataRoomCompanySchema>;
export type StoredCompany = z.infer<typeof storedCompanySchema>;
export type CompanyProfile = DataRoomCompany['profile'];
export type CompanyMetric = DataRoomCompany['metrics'][number];
export type CompanyRecord = DataRoomCompany['records'][number];
export type CompanyOwnership = NonNullable<DataRoomCompany['ownership']>;
export type HolderKind = (typeof holderKinds)[number];
export type CompanyMetricSeries = {
	metric: string;
	latest: CompanyMetric;
	points: CompanyMetric[];
	projection?: CompanyMetric;
	interim: CompanyMetric[];
};
export type CompanyFact = { field: string; value: string };
export type CompanyFactGroupName = 'company' | 'entity' | 'registration';
export type CompanyFactGroup = { group: CompanyFactGroupName; facts: CompanyFact[] };
export type MetricChange = { kind: 'percent'; percent: number } | { kind: 'turnedPositive' } | { kind: 'turnedNegative' };
export type PeriodKind = MomentPrecision;
export type RecordGroup = { category: string; records: CompanyRecord[] };

export const fundingCategories = ['funding', 'grant'];
export const proofCategories = ['certification', 'ip', 'award', 'reference'];
const headlineMetric = 'annualRevenue';

export function isEmptyCompany(company: DataRoomCompany): boolean {
	return Object.keys(company.profile).length === 0 && company.metrics.length === 0 && company.records.length === 0
		&& !company.activity && !company.ownership;
}

export function localized(text: Record<string, string> | undefined, locale: string): string {
	if (!text) return '';
	return text[locale] || text.en || Object.values(text).find(Boolean) || '';
}

export function companyFactGroups(profile: CompanyProfile, locale: string): CompanyFactGroup[] {
	const representative = [profile.representativeTitle ?? '', personName(profile.representative ?? '', locale === 'ko' ? 'ko' : 'en')]
		.filter(Boolean).join(' ');
	const groups: CompanyFactGroup[] = [
		{
			group: 'company',
			facts: [
				{ field: 'representative', value: representative },
				{ field: 'foundedDate', value: profile.foundedDate ?? '' },
				{ field: 'address', value: localized(profile.address, locale) },
				{ field: 'officeAddress', value: localized(profile.officeAddress, locale) }
			]
		},
		{
			group: 'entity',
			facts: [
				{ field: 'jurisdiction', value: localized(profile.jurisdiction, locale) },
				{ field: 'capital', value: profile.capital ? formatMoney(profile.capital, locale) : '' },
				{ field: 'fiscalYearEnd', value: fiscalYearEndLabel(profile.fiscalYearEnd, locale) },
				{ field: 'bankAccount', value: localized(profile.bankAccount, locale) }
			]
		},
		{
			group: 'registration',
			facts: Object.entries(legalAttributesIn(profile, locale)).map(([label, value]) => ({ field: label, value }))
		}
	];
	return groups
		.map((group) => ({ ...group, facts: group.facts.filter((fact) => fact.value) }))
		.filter((group) => group.facts.length > 0);
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
	return [...byName].flatMap(([metric, points]) => seriesOf(metric, points));
}

function seriesOf(metric: string, points: CompanyMetric[]): CompanyMetricSeries[] {
	const ordered = points.toSorted((left, right) => compareMomentsNewestFirst(right.period, left.period));
	const kind = ordered.some((point) => periodKind(point) === 'year') ? 'year' : periodKind(ordered[ordered.length - 1]);
	const reported = ordered.filter((point) => !isProjected(point));
	const reportedOfKind = reported.filter((point) => periodKind(point) === kind);
	const lastReported = reportedOfKind.at(-1);
	const projection = ordered.findLast((point) => isProjected(point) && periodKind(point) === kind
		&& (!lastReported || compareMomentsNewestFirst(point.period, lastReported.period) < 0));
	const latest = lastReported ?? projection;
	if (!latest) return [];
	return [{
		metric,
		latest,
		points: lastReported ? reportedOfKind : [latest],
		projection: lastReported ? projection : undefined,
		interim: kind === 'year' ? interimPoints(reported, spanOf(latest).year) : []
	}];
}

function interimPoints(reported: CompanyMetric[], closedYear: number): CompanyMetric[] {
	const open = reported.filter((point) => periodKind(point) !== 'year' && spanOf(point).year > closedYear);
	const finest = open.at(-1);
	if (!finest) return [];
	return open.filter((point) => spanOf(point).year === spanOf(finest).year && periodKind(point) === periodKind(finest));
}

function spanOf(metric: CompanyMetric): MomentSpan {
	return momentSpanOf(metric.period) ?? { precision: 'year', year: 0, part: 1, start: '', end: '' };
}

const companyClockOffsetHours = 9;

export function isProjected(metric: CompanyMetric): boolean {
	if (!metric.recordedAt) return false;
	const recordedDay = new Date(Date.parse(metric.recordedAt) + companyClockOffsetHours * 3_600_000).toISOString().slice(0, 10);
	return recordedDay < spanOf(metric).end;
}

export function periodYear(metric: CompanyMetric): number {
	return spanOf(metric).year;
}

export function periodKind(metric: CompanyMetric): PeriodKind {
	return spanOf(metric).precision;
}

export function periodLabel(metric: CompanyMetric, locale: string): string {
	return momentLabel(metric.period, locale);
}

export type MetricBar = { period: string; value: number; share: number; isLatest: boolean; isProjected: boolean };
export type MetricBarChart = { bars: MetricBar[]; positiveShare: number; negativeShare: number };

export function metricBars(series: CompanyMetricSeries, periodCount: number, locale: string): MetricBarChart {
	const reported = series.points.slice(series.projection ? 1 - periodCount : -periodCount);
	const points = series.projection ? [...reported, series.projection] : reported;
	const values = points.map(metricNumber);
	const highest = Math.max(0, ...values);
	const lowest = Math.min(0, ...values);
	const range = highest - lowest;
	const bars = points.map((point, index) => ({
		period: shortPeriodLabel(point, locale),
		value: values[index],
		share: values[index] > 0 ? values[index] / highest : values[index] < 0 ? values[index] / lowest : 0,
		isLatest: points[index] === series.latest,
		isProjected: isProjected(points[index])
	}));
	if (range === 0) return { bars, positiveShare: 1, negativeShare: 0 };
	return { bars, positiveShare: highest / range, negativeShare: -lowest / range };
}

export function subPeriodLabel(metric: CompanyMetric, locale: string): string {
	const span = spanOf(metric);
	const isKorean = locale.startsWith('ko');
	if (span.precision === 'half') return isKorean ? (span.part === 1 ? '상반기' : '하반기') : `H${span.part}`;
	if (span.precision === 'quarter') return isKorean ? `${span.part}분기` : `Q${span.part}`;
	if (span.precision === 'month') return new Intl.DateTimeFormat(locale, { month: 'short', timeZone: 'UTC' }).format(Date.parse(span.start));
	return span.start.slice(5).replace('-', '.');
}

export function shortPeriodLabel(metric: CompanyMetric, locale: string): string {
	const span = spanOf(metric);
	const shortYear = String(span.year).slice(-2);
	if (span.precision === 'year') return String(span.year);
	if (span.precision === 'month') return `${shortYear}.${span.start.slice(5, 7)}`;
	if (span.precision === 'day') return span.start.slice(5).replace('-', '.');
	const isKorean = locale.startsWith('ko');
	return isKorean ? `${shortYear}년 ${subPeriodLabel(metric, locale)}` : `${subPeriodLabel(metric, locale)} '${shortYear}`;
}

export function metricNumber(metric: CompanyMetric): number {
	return metric.amount?.amount ?? metric.value ?? 0;
}

export function changeFromPrevious(series: CompanyMetricSeries): MetricChange | undefined {
	const previous = series.points.at(-2);
	return previous ? changeBetween(previous, series.latest) : undefined;
}

export function projectedChange(series: CompanyMetricSeries): MetricChange | undefined {
	return series.projection ? changeBetween(series.latest, series.projection) : undefined;
}

function changeBetween(earlier: CompanyMetric, later: CompanyMetric): MetricChange | undefined {
	if (metricNumber(earlier) === 0) return undefined;
	const [before, after] = [metricNumber(earlier), metricNumber(later)];
	if (before < 0 && after >= 0) return { kind: 'turnedPositive' };
	if (before > 0 && after < 0) return { kind: 'turnedNegative' };
	return { kind: 'percent', percent: (after - before) / Math.abs(before) * 100 };
}

export function headlineSeries(series: CompanyMetricSeries[]): CompanyMetricSeries | undefined {
	return series.find((candidate) => candidate.metric === headlineMetric)
		?? series.find((candidate) => candidate.latest.amount !== undefined);
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
	return compareMomentsNewestFirst(left.date, right.date);
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

export function daysUntil(moment: Moment, today: Date): number {
	const lastDay = momentSpanOf(moment)?.end;
	if (!lastDay) return 0;
	return Math.ceil((Date.parse(`${lastDay}T23:59:59+09:00`) - today.getTime()) / 86_400_000);
}

export function recordAttributes(record: CompanyRecord, locale: string): [string, string][] {
	return Object.entries(record.attributes)
		.filter(([, value]) => value !== null && value !== '')
		.map(([key, value]) => [key, attributeText(value, locale)]);
}

function attributeText(value: unknown, locale: string): string {
	if (typeof value === 'string') return value;
	const money = moneySchema.safeParse(value);
	return money.success ? formatCompactMoney(money.data, locale) : JSON.stringify(value);
}
