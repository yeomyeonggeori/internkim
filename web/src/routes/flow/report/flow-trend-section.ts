// Flow 거리 추세 섹션을 계산합니다.
import { emptyTrend } from './flow-report-section-helpers';
import type { FlowReportSection, FlowReportSectionID, FlowReportSectionLabel, FlowReportTrend } from './flow-report-types';

type BuildTrendSectionInput = {
	id: FlowReportSectionID;
	labels: FlowReportSectionLabel;
	emptyLabel: string;
	trend?: FlowReportTrend;
};

export function buildTrendSection(input: BuildTrendSectionInput): FlowReportSection {
	const trend = input.trend ?? emptyTrend('km');
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
		items: [],
		rows: [],
		trend
	};
}
