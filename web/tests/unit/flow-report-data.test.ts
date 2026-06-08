import { describe, expect, test } from 'bun:test';
import { buildFlowReportSections } from '../../src/routes/flow/report/flow-report-data';
import type { FlowReportCopy, FlowReportMetrics } from '../../src/routes/flow/report/flow-report-data';
import { flowReportFixtureDefinitions, flowReportFixtureMetrics, flowReportFixtureSnapshot, flowReportFixtureTasks } from './fixtures/flow-report-summary';

const koreanReportCopy: FlowReportCopy = {
	weekdays: ['월', '화', '수', '목', '금', '토', '일'],
	fallbackType: '기타',
	fallbackBusiness: '미지정',
	teamAverageLabel: '팀 평균',
	memberScrollHint: '{count}명 전체 · 목록 안에서 스크롤',
	currentWeekTrend: '이번 주',
	previousWeekTrend: '지난 주',
	currentMonthTrend: '이번 달',
	previousMonthTrend: '지난 달',
	monthlyDayLabelTemplate: '{day}일'
};

describe('buildFlowReportSections', () => {
	test('builds sorted report chart sections from flow metrics', () => {
		const sections = buildFlowReportSections(flowReportFixtureMetrics, {
			emptyLabel: '이번 주 데이터 없음',
			sectionLabels: {
				weeklyStatus: {
					title: '주간 상태',
					description: '상태별 업무 분포와 막힌 일을 빠르게 확인합니다.'
				},
				memberDistance: {
					title: '구성원 거리',
					description: '완료와 진행 업무 기준의 주간 업무 거리입니다.'
				},
				weeklyDistanceTrend: {
					title: '주간 통계',
					description: '지난주와 이번주의 일별 팀 거리 합을 비교합니다.'
				},
				monthlyDistanceTrend: {
					title: '월간 통계',
					description: '지난달과 이번달의 일별 팀 거리 합을 비교합니다.'
				},
				businessDistance: {
					title: '대분류별 업무',
					description: ''
				}
			},
			copy: koreanReportCopy,
			report: flowReportFixtureSnapshot,
			tasks: flowReportFixtureTasks,
			definitions: flowReportFixtureDefinitions,
			weekStartISO: '2026-06-01'
		});

		expect(sections.weeklyStatus.chartKind).toBe('dailyTypeStacked');
		expect(sections.weeklyStatus.total).toBe(7);
		expect(sections.weeklyStatus.unit).toBe('km');
		expect(sections.weeklyStatus.rows.map((row) => row.label)).toEqual(['월', '화', '수', '목', '금', '토', '일']);
		expect(sections.weeklyStatus.rows.map((row) => row.total)).toEqual([5, 0, 2, 0, 0, 0, 0]);
		expect(sections.weeklyStatus.rows[0].segments).toEqual([
			{ label: '구현', value: 5, percent: 100, tone: 'type', colorIndex: 0 }
		]);
		expect(sections.weeklyStatus.items.map((item) => [item.label, item.value])).toEqual([
			['구현', 5],
			['회의', 2]
		]);
		expect(sections.weeklyStatus.items.map((item) => [item.label, item.colorIndex])).toEqual([
			['구현', 0],
			['회의', 1]
		]);

		expect(sections.memberDistance.chartKind).toBe('memberTypeStacked');
		expect(sections.memberDistance.unit).toBe('km');
		expect(sections.memberDistance.total).toBe(7);
		expect(sections.memberDistance.averageValue).toBe(4);
		expect(sections.memberDistance.rows.map((row) => row.label)).toEqual(['김여명', '박예시']);
		expect(sections.memberDistance.rows[0]).toMatchObject({ label: '김여명', total: 5, percent: 71 });
		expect(sections.memberDistance.rows[0].segments.map((segment) => [segment.label, segment.value])).toEqual([
			['구현', 5]
		]);

		expect(sections.weeklyDistanceTrend.chartKind).toBe('lineComparison');
		expect(sections.weeklyDistanceTrend.total).toBe(18);
		expect(sections.weeklyDistanceTrend.alertValue).toBe(7);
		expect(sections.weeklyDistanceTrend.trend.currentValues).toEqual([0, 7, 10, 10, 15, 18, 18]);

		expect(sections.monthlyDistanceTrend.chartKind).toBe('lineComparison');
		expect(sections.monthlyDistanceTrend.total).toBe(18);
		expect(sections.monthlyDistanceTrend.alertValue).toBe(7);
		expect(sections.monthlyDistanceTrend.trend.currentValues).toEqual([0, 6, 9, 12, 15, 18, 18]);

		expect(sections.businessDistance.chartKind).toBe('donut');
		expect(sections.businessDistance.unit).toBe('km');
		expect(sections.businessDistance.total).toBe(7);
		expect(sections.businessDistance.items.map((item) => [item.label, item.value])).toEqual([
			['여명거리', 5],
			['김인턴', 2]
		]);
		expect(sections.businessDistance.items[0]).toMatchObject({ tone: 'business', percent: 71 });
	});

	test('keeps empty report sections explicit when metrics have no values', () => {
		const sections = buildFlowReportSections(
			{
				totalTasks: 0,
				completedTasks: 0,
				requestedTasks: 0,
				pausedTasks: 0,
				stoppedTasks: 0,
				totalDistance: 0,
				statusCounts: {},
				businessCounts: {},
				typeCounts: {},
				memberDistances: {}
			},
			{
				emptyLabel: '이번 주 데이터 없음',
				sectionLabels: {
					weeklyStatus: { title: '주간 상태', description: '' },
					memberDistance: { title: '구성원 거리', description: '' },
					weeklyDistanceTrend: { title: '주간 통계', description: '' },
					monthlyDistanceTrend: { title: '월간 통계', description: '' },
					businessDistance: { title: '대분류별 업무', description: '' }
				},
				copy: koreanReportCopy
			}
		);

		expect(sections.weeklyStatus.rows.length).toBe(7);
		expect(sections.weeklyStatus.rows.every((row) => row.total === 0 && row.segments.length === 0)).toBe(true);
		expect(sections.weeklyStatus.emptyLabel).toBe('이번 주 데이터 없음');
		expect(sections.memberDistance.total).toBe(0);
		expect(sections.weeklyDistanceTrend.total).toBe(0);
		expect(sections.monthlyDistanceTrend.total).toBe(0);
		expect(sections.businessDistance.maxValue).toBe(1);
	});

	test('reads legacy score metrics as distance metrics', () => {
		const { totalDistance, memberDistances, ...legacyMetricBase } = flowReportFixtureMetrics;
		const legacyMetrics: FlowReportMetrics = {
			...legacyMetricBase,
			totalScore: totalDistance,
			memberScores: memberDistances
		};

		const sections = buildFlowReportSections(legacyMetrics, {
			emptyLabel: '이번 주 데이터 없음',
			sectionLabels: {
				weeklyStatus: { title: '주간 상태', description: '' },
				memberDistance: { title: '구성원 거리', description: '' },
				weeklyDistanceTrend: { title: '주간 통계', description: '' },
				monthlyDistanceTrend: { title: '월간 통계', description: '' },
				businessDistance: { title: '대분류별 업무', description: '' }
			},
			copy: koreanReportCopy,
			tasks: flowReportFixtureTasks,
			definitions: flowReportFixtureDefinitions
		});

		expect(sections.memberDistance.total).toBe(7);
		expect(sections.memberDistance.averageValue).toBe(4);
		expect(sections.memberDistance.rows.map((row) => row.label)).toEqual(['김여명', '박예시']);
	});

	test('localizes report system labels without translating user definitions', () => {
		const englishOptions = {
			emptyLabel: 'No data for this week',
			sectionLabels: {
				weeklyStatus: { title: 'Weekly daily type distance', description: '' },
				memberDistance: { title: 'Member distance', description: '' },
				weeklyDistanceTrend: { title: 'Weekly statistics', description: '' },
				monthlyDistanceTrend: { title: 'Monthly statistics', description: '' },
				businessDistance: { title: 'Weekly business distance', description: '' }
			},
			copy: {
				weekdays: ['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun'],
				fallbackType: 'Other',
				fallbackBusiness: 'Unassigned',
				teamAverageLabel: 'Team average',
				memberScrollHint: '{count} members total · scroll inside list',
				currentWeekTrend: 'This week',
				previousWeekTrend: 'Last week',
				currentMonthTrend: 'This month',
				previousMonthTrend: 'Last month',
				monthlyDayLabelTemplate: 'Day {day}'
			},
			report: flowReportFixtureSnapshot,
			tasks: [
				...flowReportFixtureTasks,
				{ participantNames: ['정의'], business: '', type: '구현', size: 'S', status: '완료', startDate: '2026-06-01', endDate: '2026-06-01' }
			],
			definitions: flowReportFixtureDefinitions,
			weekStartISO: '2026-06-01'
		};

		const sections = buildFlowReportSections(flowReportFixtureMetrics, englishOptions);

		expect(sections.weeklyStatus.rows.map((row) => row.label)).toEqual(['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun']);
		expect(sections.weeklyStatus.items[0].label).toBe('구현');
		expect(sections.businessDistance.items.map((item) => item.label)).toEqual(['여명거리', '김인턴', 'Unassigned']);
		expect(sections.memberDistance.rows[0].segments.map((segment) => segment.label)).toEqual(['구현']);
		expect(sections.memberDistance.teamAverageLabel).toBe('Team average');
		expect(sections.memberDistance.memberScrollHint).toBe('{count} members total · scroll inside list');
		expect(sections.weeklyDistanceTrend.trend.labels).toEqual(['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun']);
		expect(sections.weeklyDistanceTrend.trend.currentLabel).toBe('This week');
		expect(sections.weeklyDistanceTrend.trend.previousLabel).toBe('Last week');
		expect(sections.monthlyDistanceTrend.trend.currentLabel).toBe('This month');
		expect(sections.monthlyDistanceTrend.trend.previousLabel).toBe('Last month');
		expect(sections.monthlyDistanceTrend.trend.labelTemplate).toBe('Day {day}');
	});
});
