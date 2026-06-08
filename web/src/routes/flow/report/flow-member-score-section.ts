import { emptyTrend, percentage } from './flow-report-section-helpers';
import type { FlowReportCopy, FlowReportRow, FlowReportSection, FlowReportSectionID, FlowReportSectionLabel } from './flow-report-types';

type BuildMemberScoreSectionInput = {
	id: FlowReportSectionID;
	labels: FlowReportSectionLabel;
	emptyLabel: string;
	copy: FlowReportCopy;
	memberScores: Record<string, number>;
	averageValue?: number;
};

export function buildMemberScoreSection(input: BuildMemberScoreSectionInput): FlowReportSection {
	const rows = buildMemberScoreRows(input.memberScores, input.copy.memberScoreLabel);
	const total = rows.reduce((sum, row) => sum + row.total, 0);
	const maxValue = Math.max(1, ...rows.map((row) => row.total));

	return {
		id: input.id,
		chartKind: 'memberScoreList',
		title: input.labels.title,
		description: input.labels.description,
		unit: input.copy.scoreUnit,
		total,
		maxValue,
		averageValue: input.averageValue ?? 0,
		alertValue: 0,
		emptyLabel: input.emptyLabel,
		teamAverageLabel: input.copy.teamAverageLabel,
		memberScrollHint: input.copy.memberScrollHint,
		items: [],
		trend: emptyTrend(input.copy.scoreUnit),
		rows: rows.map((row) => ({ ...row, percent: percentage(row.total, total) }))
	};
}

function buildMemberScoreRows(memberScores: Record<string, number>, scoreLabel: string): FlowReportRow[] {
	return Object.entries(memberScores)
		.map(([memberName, score]) => ({
			label: memberName,
			total: score,
			percent: 0,
			segments: [
				{
					label: scoreLabel,
					value: score,
					percent: 100,
					tone: 'type' as const,
					colorIndex: 0
				}
			]
		}))
		.filter((row) => row.total > 0)
		.sort((left, right) => right.total - left.total || left.label.localeCompare(right.label));
}
