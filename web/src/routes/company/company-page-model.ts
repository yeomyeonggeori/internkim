export type CompanyShareProfile = {
	name: string;
	brandName?: string;
	slogan?: string;
	description?: string;
	website?: string;
	foundedDate?: string;
	employeeCount?: number;
	jurisdiction?: string;
	representative?: string;
	representativeTitle?: string;
	capital?: string;
	fiscalYearEnd?: string;
	email?: string;
};

export type CompanyShareMetric = {
	metric: string;
	year: number;
	quarter?: number;
	month?: number;
	value: number;
	currency?: CompanyMetricCurrency;
	valueUSD?: number;
	unit?: string;
	source?: string;
};

export type CompanyMetricCurrency = 'USD' | 'KRW' | 'EUR' | 'JPY' | 'GBP' | 'CNY' | 'HKD' | 'SGD' | 'AUD' | 'CAD' | 'CHF' | 'INR';

export type CompanyMetricDisplayCurrency = 'USD' | 'local';

export type CompanyShareRecord = {
	category: string;
	date?: string;
	title: string;
	titles?: Record<string, string>;
	descriptions?: Record<string, string>;
	attributes?: Record<string, string>;
};

export type CompanyShareDocument = {
	documentType: string;
	title: string;
	language?: string;
	summary?: string;
	issuedAt?: string;
};

export type CompanyShareNarrative = {
	highlights: string[];
	businessModel?: string;
	customerEvidence?: string;
	marketOpportunity?: string;
	competitiveAdvantage?: string;
	roadmap?: string;
	fundingStage?: string;
	fundingTarget?: string;
	useOfFunds?: string;
};

export type CompanyShareMetricContext = {
	labels: Record<string, string>;
	descriptions: Record<string, string>;
	favorableDirection: 'increase' | 'decrease' | 'neutral';
	evidenceRole?: '' | 'growth' | 'efficiency' | 'scale' | 'quality' | 'reach' | 'capital';
	showSource?: boolean;
};

export type CompanyShareActivityDay = {
	date: string;
	attendanceCount: number;
	workCount: number;
};

export type CompanyShareWorkStatus = {
	status: 'planned' | 'inProgress' | 'completed' | 'paused' | 'closed';
	count: number;
};

export type CompanyShareRecentWork = {
	memberSeed: string;
	title: string;
	business?: string;
	type?: string;
	size?: string;
	status: 'inProgress' | 'completed';
	startDate?: string;
	endDate?: string;
	date: string;
};

export type CompanyShareMember = {
	seed: string;
	surname: string;
	jobTitle?: string;
	image?: string;
};

export type CompanyShareTeamActivity = {
	windowDays: number;
	members: CompanyShareMember[];
	days: CompanyShareActivityDay[];
	workStatuses: CompanyShareWorkStatus[];
	recentWork: CompanyShareRecentWork[];
	attendanceTotal: number;
	workTotal: number;
};

export type CompanyShareSnapshot = {
	revision: number;
	publishedAt: string;
	profiles: Record<string, CompanyShareProfile>;
	metrics: CompanyShareMetric[];
	primaryMetric?: string;
	metricContexts?: Record<string, CompanyShareMetricContext>;
	records: CompanyShareRecord[];
	documents?: CompanyShareDocument[];
	contactEmail?: string;
	teamActivity?: CompanyShareTeamActivity;
	narratives?: Record<string, CompanyShareNarrative>;
};

export function latestCompanyMetrics(metrics: CompanyShareMetric[]): CompanyShareMetric[] {
	const latestByName = new Map<string, CompanyShareMetric>();
	for (const metric of metrics) {
		const current = latestByName.get(metric.metric);
		if (!current || companyMetricPeriod(metric) > companyMetricPeriod(current)) latestByName.set(metric.metric, metric);
	}
	return [...latestByName.values()].sort((left, right) => left.metric.localeCompare(right.metric));
}

export function companyMetricSeries(metrics: CompanyShareMetric[], metricName: string): CompanyShareMetric[] {
	return metrics.filter((metric) => metric.metric === metricName).sort((left, right) => companyMetricPeriod(left) - companyMetricPeriod(right));
}

export function companyMetricPeriod(metric: CompanyShareMetric): number {
	return metric.year * 100 + (metric.month ?? (metric.quarter ? metric.quarter * 3 : 0));
}

export function companyMetricPeriodLabel(metric: CompanyShareMetric, language = 'en'): string {
	if (metric.month) return `${metric.year}.${String(metric.month).padStart(2, '0')}`;
	if (metric.quarter) return language === 'ko' ? `${metric.year}년 ${metric.quarter}분기` : `${metric.year} Q${metric.quarter}`;
	return String(metric.year);
}

export function companyMetricChangePercentage(series: CompanyShareMetric[], displayCurrency: CompanyMetricDisplayCurrency): number | undefined {
	if (series.length < 2) return undefined;
	const previousMetric = series.at(-2);
	const currentMetric = series.at(-1);
	if (!previousMetric || !currentMetric) return undefined;
	const previousValue = companyMetricDisplayValue(previousMetric, displayCurrency);
	if (previousValue === 0) return undefined;
	const currentValue = companyMetricDisplayValue(currentMetric, displayCurrency);
	return (currentValue - previousValue) / Math.abs(previousValue) * 100;
}

export function companyMetricLabel(metricName: string): string {
	return metricName
		.replace(/[_-]+/g, ' ')
		.replace(/([a-z0-9])([A-Z])/g, '$1 $2')
		.replace(/^\w/, (character) => character.toUpperCase());
}

export function companyRecordTitle(record: CompanyShareRecord, language: string): string {
	return record.titles?.[language] || record.title;
}

export function companyRecordDescription(record: CompanyShareRecord, language: string): string {
	return record.descriptions?.[language] || '';
}

export function companyMetricChangeAssessment(change: number | undefined, favorableDirection: CompanyShareMetricContext['favorableDirection']): 'favorable' | 'unfavorable' | 'neutral' {
	if (change === undefined || change === 0 || favorableDirection === 'neutral') return 'neutral';
	if (favorableDirection === 'increase') return change > 0 ? 'favorable' : 'unfavorable';
	return change < 0 ? 'favorable' : 'unfavorable';
}

export function companyActivityIntensity(day: CompanyShareActivityDay, days: CompanyShareActivityDay[]): number {
	const value = day.attendanceCount + day.workCount;
	const maximum = Math.max(...days.map((candidate) => candidate.attendanceCount + candidate.workCount), 0);
	if (value === 0 || maximum === 0) return 0;
	return Math.max(1, Math.ceil(value / maximum * 4));
}

export function companyWorkStatusPercentage(count: number, statuses: CompanyShareWorkStatus[]): number {
	const total = statuses.reduce((sum, status) => sum + status.count, 0);
	return total === 0 ? 0 : count / total * 100;
}

export function companyLocalCurrency(metrics: CompanyShareMetric[]): CompanyMetricCurrency | undefined {
	const currencies = new Set(
		metrics
			.filter((metric) => metric.currency && metric.currency !== 'USD' && metric.valueUSD !== undefined)
			.map((metric) => metric.currency)
	);
	if (currencies.size !== 1) return undefined;
	return currencies.values().next().value;
}

export function companyMetricDisplayValue(metric: CompanyShareMetric, displayCurrency: CompanyMetricDisplayCurrency): number {
	if (displayCurrency === 'USD' && metric.valueUSD !== undefined) return metric.valueUSD;
	return metric.value;
}

export function formatCompanyMetricValue(metric: CompanyShareMetric, displayCurrency: CompanyMetricDisplayCurrency, language: string): string {
	const currency = companyMetricDisplayCode(metric, displayCurrency);
	const value = companyMetricDisplayValue(metric, displayCurrency);
	if (currency) {
		return new Intl.NumberFormat(currency === 'USD' ? 'en-US' : language, {
			style: 'currency',
			currency,
			currencyDisplay: 'narrowSymbol',
			notation: 'compact',
			maximumFractionDigits: 1
		}).format(value);
	}
	return `${new Intl.NumberFormat(language).format(value)}${metric.unit ? ` ${metric.unit}` : ''}`;
}

function companyMetricDisplayCode(metric: CompanyShareMetric, displayCurrency: CompanyMetricDisplayCurrency): CompanyMetricCurrency | undefined {
	if (displayCurrency === 'USD' && metric.valueUSD !== undefined) return 'USD';
	return metric.currency;
}

export function isCompanyShareSnapshot(value: unknown): value is CompanyShareSnapshot {
	if (!value || typeof value !== 'object') return false;
	const candidate = value as Record<string, unknown>;
	return typeof candidate.revision === 'number' && typeof candidate.publishedAt === 'string' && isProfileRecord(candidate.profiles) && Array.isArray(candidate.metrics) && Array.isArray(candidate.records);
}

function isProfileRecord(value: unknown): value is Record<string, CompanyShareProfile> {
	if (!value || typeof value !== 'object') return false;
	return Object.values(value).every((profile) => {
		if (!profile || typeof profile !== 'object') return false;
		return typeof (profile as Record<string, unknown>).name === 'string';
	});
}
