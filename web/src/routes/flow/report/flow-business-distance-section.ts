// Flow 사업별 거리 섹션을 계산합니다.
import { taskTeamDistance } from './flow-report-distance';
import { emptyTrend, percentage } from './flow-report-section-helpers';
import type { FlowReportDefinitions, FlowReportSection, FlowReportSectionID, FlowReportSectionLabel, FlowReportTask } from './flow-report-types';

type BuildBusinessDistanceSectionInput = {
	id: FlowReportSectionID;
	labels: FlowReportSectionLabel;
	emptyLabel: string;
	tasks: FlowReportTask[];
	definitions: FlowReportDefinitions;
};

export function buildBusinessDistanceSection(input: BuildBusinessDistanceSectionInput): FlowReportSection {
	const values = new Map<string, number>();
	for (const task of input.tasks) {
		const distance = taskTeamDistance(task, input.definitions);
		if (distance <= 0) continue;

		const business = task.business.trim() || '미지정';
		values.set(business, (values.get(business) ?? 0) + distance);
	}

	const entries = sortedBusinessDistances(values, input.definitions.categories);
	const total = entries.reduce((sum, [, value]) => sum + value, 0);
	const maxValue = Math.max(1, ...entries.map(([, value]) => value));

	return {
		id: input.id,
		chartKind: 'donut',
		title: input.labels.title,
		description: input.labels.description,
		unit: 'km',
		total,
		maxValue,
		averageValue: 0,
		alertValue: 0,
		emptyLabel: input.emptyLabel,
		rows: [],
		trend: emptyTrend('km'),
		items: entries.map(([label, value]) => ({
			label,
			description: '',
			value,
			percent: percentage(value, total),
			tone: 'business'
		}))
	};
}

function sortedBusinessDistances(businessDistances: Map<string, number>, businessOrder: string[]): [string, number][] {
	return Array.from(businessDistances.entries()).sort((left, right) => {
		const orderDifference = businessIndex(left[0], businessOrder) - businessIndex(right[0], businessOrder);
		if (orderDifference !== 0) return orderDifference;

		const valueDifference = right[1] - left[1];
		if (valueDifference !== 0) return valueDifference;

		return left[0].localeCompare(right[0]);
	});
}

function businessIndex(businessName: string, businessOrder: string[]): number {
	const index = businessOrder.indexOf(businessName);
	return index === -1 ? businessOrder.length : index;
}
