import { distanceForTaskProgress } from './flow-report-distance';
import { emptyTrend, incrementNested, percentage, sortedTypeDistances, typeIndex } from './flow-report-section-helpers';
import type { FlowReportCopy, FlowReportDefinitions, FlowReportRow, FlowReportSection, FlowReportSectionID, FlowReportSectionLabel, FlowReportTask } from './flow-report-types';

type BuildMemberDistanceSectionInput = {
	id: FlowReportSectionID;
	labels: FlowReportSectionLabel;
	emptyLabel: string;
	copy: FlowReportCopy;
	tasks: FlowReportTask[];
	definitions: FlowReportDefinitions;
	memberDistances?: Record<string, number>;
	averageValue?: number;
};

export function buildMemberDistanceSection(input: BuildMemberDistanceSectionInput): FlowReportSection {
	const rowTotals = new Map<string, Map<string, number>>();
	for (const task of input.tasks) {
		const distance = distanceForTaskProgress(task, input.definitions);
		if (distance <= 0) continue;
		for (const participantName of task.participantNames) {
			incrementNested(rowTotals, participantName, task.type, distance);
		}
	}

	if (input.memberDistances) {
		reconcileMemberDistanceRows(rowTotals, input.memberDistances, input.copy.fallbackType);
	}

	const rows = buildMemberDistanceRows(rowTotals, [...input.definitions.types, input.copy.fallbackType]);
	const total = rows.reduce((sum, row) => sum + row.total, 0);
	const maxValue = Math.max(1, ...rows.map((row) => row.total));

	return {
		id: input.id,
		chartKind: 'memberTypeStacked',
		title: input.labels.title,
		description: input.labels.description,
		unit: 'km',
		total,
		maxValue,
		averageValue: input.averageValue ?? 0,
		alertValue: 0,
		emptyLabel: input.emptyLabel,
		teamAverageLabel: input.copy.teamAverageLabel,
		memberScrollHint: input.copy.memberScrollHint,
		items: [],
		trend: emptyTrend('km'),
		rows: rows.map((row) => ({ ...row, percent: percentage(row.total, total) }))
	};
}

function buildMemberDistanceRows(rowTotals: Map<string, Map<string, number>>, typeOrder: string[]): FlowReportRow[] {
	return Array.from(rowTotals.entries())
		.map(([memberName, typeDistances]) => {
			const total = Array.from(typeDistances.values()).reduce((sum, value) => sum + value, 0);
			return {
				label: memberName,
				total,
				percent: 0,
				segments: sortedTypeDistances(typeDistances, typeOrder).map(([typeName, value]) => ({
					label: typeName,
					value,
					percent: percentage(value, total),
					tone: 'type' as const,
					colorIndex: typeIndex(typeName, typeOrder)
				}))
			};
		})
		.filter((row) => row.total > 0)
		.sort((left, right) => right.total - left.total || left.label.localeCompare(right.label));
}

function reconcileMemberDistanceRows(rowTotals: Map<string, Map<string, number>>, memberDistances: Record<string, number>, fallbackType: string): void {
	for (const [memberName, totalDistance] of Object.entries(memberDistances)) {
		if (totalDistance <= 0) continue;

		const typeDistances = rowTotals.get(memberName) ?? new Map<string, number>();
		const knownDistance = Array.from(typeDistances.values()).reduce((sum, distance) => sum + distance, 0);
		const remainingDistance = Math.max(0, totalDistance - knownDistance);
		if (remainingDistance > 0) {
			typeDistances.set(fallbackType, (typeDistances.get(fallbackType) ?? 0) + remainingDistance);
		}
		rowTotals.set(memberName, typeDistances);
	}
}
