export type FlowReportMetrics = {
	totalTasks: number;
	completedTasks: number;
	requestedTasks: number;
	pausedTasks: number;
	stoppedTasks: number;
	totalScore: number;
	statusCounts: Record<string, number>;
	businessCounts: Record<string, number>;
	typeCounts: Record<string, number>;
	memberScores: Record<string, number>;
};

export type FlowReportSectionID = 'weeklyStatus' | 'memberScores' | 'businessDistance' | 'typeBreakdown';
export type FlowReportChartKind = 'stacked' | 'workload' | 'donut' | 'treemap';

export type FlowReportSectionLabel = {
	title: string;
	description: string;
};

export type FlowReportSectionLabels = Record<FlowReportSectionID, FlowReportSectionLabel>;

export type FlowReportOptions = {
	statusLabels: Record<string, string>;
	statusDescriptions: Record<string, string>;
	emptyLabel: string;
	sectionLabels: FlowReportSectionLabels;
};

export type FlowReportItem = {
	label: string;
	description: string;
	value: number;
	percent: number;
	tone: FlowReportTone;
};

export type FlowReportTone = 'success' | 'active' | 'request' | 'planned' | 'paused' | 'stopped' | 'member' | 'business' | 'type';

export type FlowReportSection = {
	id: FlowReportSectionID;
	chartKind: FlowReportChartKind;
	title: string;
	description: string;
	unit: string;
	total: number;
	maxValue: number;
	averageValue: number;
	alertValue: number;
	emptyLabel: string;
	items: FlowReportItem[];
};

export type FlowReportSections = Record<FlowReportSectionID, FlowReportSection>;

export function buildFlowReportSections(metrics: FlowReportMetrics, options: FlowReportOptions): FlowReportSections {
	return {
		weeklyStatus: buildSection({
			id: 'weeklyStatus',
			chartKind: 'stacked',
			labels: options.sectionLabels.weeklyStatus,
			values: metrics.statusCounts,
			emptyLabel: options.emptyLabel,
			unit: '개',
			alertValue: metrics.pausedTasks + metrics.stoppedTasks,
			labelValue: (label) => options.statusLabels[label] ?? label,
			descriptionValue: (label) => options.statusDescriptions[label] ?? '',
			toneValue: statusTone
		}),
		memberScores: buildSection({
			id: 'memberScores',
			chartKind: 'workload',
			labels: options.sectionLabels.memberScores,
			values: metrics.memberScores,
			emptyLabel: options.emptyLabel,
			unit: 'km',
			averageValue: averagePositiveValue(metrics.memberScores),
			labelValue: (label) => label,
			descriptionValue: () => '',
			toneValue: () => 'member'
		}),
		businessDistance: buildSection({
			id: 'businessDistance',
			chartKind: 'donut',
			labels: options.sectionLabels.businessDistance,
			values: metrics.businessCounts,
			emptyLabel: options.emptyLabel,
			unit: '개',
			labelValue: (label) => label,
			descriptionValue: () => '',
			toneValue: () => 'business'
		}),
		typeBreakdown: buildSection({
			id: 'typeBreakdown',
			chartKind: 'treemap',
			labels: options.sectionLabels.typeBreakdown,
			values: metrics.typeCounts,
			emptyLabel: options.emptyLabel,
			unit: '개',
			labelValue: (label) => label,
			descriptionValue: () => '',
			toneValue: () => 'type'
		})
	};
}

type BuildSectionInput = {
	id: FlowReportSectionID;
	chartKind: FlowReportChartKind;
	labels: FlowReportSectionLabel;
	values: Record<string, number>;
	emptyLabel: string;
	unit: string;
	averageValue?: number;
	alertValue?: number;
	labelValue: (label: string) => string;
	descriptionValue: (label: string) => string;
	toneValue: (label: string) => FlowReportTone;
};

function buildSection(input: BuildSectionInput): FlowReportSection {
	const entries = sortedPositiveEntries(input.values);
	const total = entries.reduce((sum, [, value]) => sum + value, 0);
	const maxValue = Math.max(1, ...entries.map(([, value]) => value));

	return {
		id: input.id,
		chartKind: input.chartKind,
		title: input.labels.title,
		description: input.labels.description,
		unit: input.unit,
		total,
		maxValue,
		averageValue: input.averageValue ?? 0,
		alertValue: input.alertValue ?? 0,
		emptyLabel: input.emptyLabel,
		items: entries.map(([label, value]) => ({
			label: input.labelValue(label),
			description: input.descriptionValue(label),
			value,
			percent: percentage(value, total),
			tone: input.toneValue(label)
		}))
	};
}

function sortedPositiveEntries(values: Record<string, number>): [string, number][] {
	return Object.entries(values)
		.filter(([, value]) => value > 0)
		.sort((left, right) => right[1] - left[1]);
}

function percentage(value: number, total: number): number {
	if (total <= 0) return 0;
	return Math.round((value / total) * 100);
}

function averagePositiveValue(values: Record<string, number>): number {
	const positiveValues = Object.values(values).filter((value) => value > 0);
	if (positiveValues.length === 0) return 0;
	const total = positiveValues.reduce((sum, value) => sum + value, 0);
	return Math.round(total / positiveValues.length);
}

function statusTone(status: string): FlowReportTone {
	switch (status) {
		case '완료':
			return 'success';
		case '진행':
			return 'active';
		case '요청':
			return 'request';
		case '예정':
			return 'planned';
		case '일시정지':
			return 'paused';
		case '중단':
		case '기각':
			return 'stopped';
		default:
			return 'planned';
	}
}
