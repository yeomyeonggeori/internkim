import { emptyTrend, percentage } from './task-report-section-helpers';
import type { TaskMemberScoreSection, TaskReportCopy, TaskReportMemberScoreDetail, TaskReportRow, TaskReportSectionLabel } from './task-report-types';

type BuildMemberScoreSectionInput = {
	id: TaskMemberScoreSection['id'];
	labels: TaskReportSectionLabel;
	emptyLabel: string;
	copy: TaskReportCopy;
	memberScores: Record<string, number>;
	memberScoreDetails: Record<string, TaskReportMemberScoreDetail>;
	averageValue?: number;
};

export function buildMemberScoreSection(input: BuildMemberScoreSectionInput): TaskMemberScoreSection {
	const rows = buildMemberScoreRows(input.memberScores, input.memberScoreDetails, input.copy);
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

function buildMemberScoreRows(memberScores: Record<string, number>, memberScoreDetails: Record<string, TaskReportMemberScoreDetail>, copy: TaskReportCopy): TaskReportRow[] {
	return Object.entries(memberScores)
		.map(([memberName, score]) => ({
			label: memberName,
			total: score,
			percent: 0,
			summary: memberScoreSummary(memberScoreDetails[memberName], copy),
			segments: [
				{
					label: copy.memberScoreLabel,
					value: score,
					percent: 100,
					tone: 'type' as const,
					colorIndex: 0
				}
			]
		}))
		.sort((left, right) => right.total - left.total || left.label.localeCompare(right.label));
}

function memberScoreSummary(detail: TaskReportMemberScoreDetail | undefined, copy: TaskReportCopy): string | undefined {
	if (!detail) return undefined;
	return `${copy.weeklyScoreLabel} ${detail.weeklyScore}${copy.scoreUnit} · ${copy.monthlyScoreLabel} ${detail.monthlyScore}${copy.scoreUnit}`;
}
