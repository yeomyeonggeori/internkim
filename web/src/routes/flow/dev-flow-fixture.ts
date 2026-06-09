import { buildFlowWeek } from './dev-flow-fixture-date';
import { devFlowMembers, devFlowSizes, devFlowStatuses, devFlowTypes } from './dev-flow-fixture-data';
import { buildDevFlowMemberScoreDetails, currentDevFlowMemberScores } from './dev-flow-fixture-score';
import { createDevFlowScoreTasks } from './dev-flow-fixture-score-tasks';
import {
	applyDevFlowMemberScores,
	buildDevFlowMetrics,
	calculateDevFlowMemberDistances,
	type DevFlowMetrics
} from './dev-flow-fixture-metrics';
import { buildDevFlowReportSnapshot } from './dev-flow-fixture-report';
import { createFixtureTasks } from './dev-flow-fixture-tasks';
import type { FlowDefinitions, FlowMember, FlowSummary } from './flow-types';
import type { FlowReportSnapshot } from './report/flow-report-data';

export type DevFlowSummary = Omit<FlowSummary, 'metrics' | 'report'> & {
	metrics: DevFlowMetrics;
	report: FlowReportSnapshot;
};

export function createDevFlowSummary(weekCode: string | null | undefined, currentUserEmail = 'admin@example.com'): DevFlowSummary {
	const week = buildFlowWeek(weekCode);
	const tasks = createFixtureTasks(week);
	const definitions: FlowDefinitions = { categories: ['여명거리', '김인턴'], types: devFlowTypes, sizes: devFlowSizes };
	const memberScoreDetails = buildDevFlowMemberScoreDetails(createDevFlowScoreTasks(week, createFixtureTasks), devFlowMembers, definitions, week);
	const memberScores = currentDevFlowMemberScores(memberScoreDetails);
	const memberDistances = calculateDevFlowMemberDistances(devFlowMembers, tasks, definitions);
	const members = applyDevFlowMemberScores(memberDistances, memberScores);

	return {
		week,
		members,
		tasks,
		metrics: buildDevFlowMetrics(tasks, definitions, memberScores, memberScoreDetails),
		definitions,
		report: buildDevFlowReportSnapshot(tasks, members, definitions, week),
		statusOptions: devFlowStatuses,
		currentUserEmail,
		currentUserName: memberNameForEmail(members, currentUserEmail),
		isAdmin: true,
		source: 'dev-mock'
	};
}

function memberNameForEmail(members: FlowMember[], email: string): string {
	return members.find((member) => member.email === email)?.name ?? email.split('@')[0] ?? '';
}
