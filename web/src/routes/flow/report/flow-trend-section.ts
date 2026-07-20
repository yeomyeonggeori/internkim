import { emptyTrend } from './flow-report-section-helpers';
import type { FlowReportCopy, FlowReportSectionLabel, FlowReportTrend, FlowTrendSection } from './flow-report-types';

type BuildTrendSectionInput = {
	id: FlowTrendSection['id'];
	labels: FlowReportSectionLabel;
	emptyLabel: string;
	copy: FlowReportCopy;
	trend?: FlowReportTrend;
};

export function buildTrendSection(input: BuildTrendSectionInput): FlowTrendSection {
	const trend = localizedTrend(input.trend ?? emptyTrend('km'), input.id, input.copy);
	const maxValue = Math.max(1, ...trend.currentValues, ...trend.previousValues);

	return {
		id: input.id,
		chartKind: 'lineComparison',
		title: input.labels.title,
		description: input.labels.description,
		unit: trend.unit,
		total: trend.currentTotal,
		maxValue,
		averageValue: 0,
		alertValue: trend.currentTotal - trend.previousTotal,
		emptyLabel: input.emptyLabel,
		teamAverageLabel: input.copy.teamAverageLabel,
		memberScrollHint: input.copy.memberScrollHint,
		items: [],
		rows: [],
		trend
	};
}

function localizedTrend(trend: FlowReportTrend, id: FlowTrendSection['id'], copy: FlowReportCopy): FlowReportTrend {
	if (id === 'weeklyDistanceTrend') {
		return {
			...trend,
			labels: copy.weekdays.length === 7 ? copy.weekdays : trend.labels,
			currentLabel: copy.currentWeekTrend,
			previousLabel: copy.previousWeekTrend
		};
	}
	if (id === 'monthlyDistanceTrend') {
		return {
			...trend,
			currentLabel: copy.currentMonthTrend,
			previousLabel: copy.previousMonthTrend,
			labelTemplate: copy.monthlyDayLabelTemplate
		};
	}
	return trend;
}
