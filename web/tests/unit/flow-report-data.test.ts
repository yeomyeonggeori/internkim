import { describe, expect, test } from 'bun:test';
import { buildFlowReportSections } from '../../src/routes/flow/report/flow-report-data';
import type { FlowReportCopy, FlowReportMetrics } from '../../src/routes/flow/report/flow-report-data';
import { flowReportFixtureDefinitions, flowReportFixtureMembers, flowReportFixtureMetrics, flowReportFixtureSnapshot, flowReportFixtureTasks } from './fixtures/flow-report-summary';

const koreanReportCopy: FlowReportCopy = {
	weekdays: ['월', '화', '수', '목', '금', '토', '일'],
	fallbackType: '기타',
	fallbackBusiness: '미지정',
	memberScoreLabel: '점수',
	weeklyScoreLabel: '주간',
	monthlyScoreLabel: '월간',
	scoreUnit: '점',
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
					title: '구성원 점수',
					description: '최근 5주와 5개월의 완료 거리 가중 점수입니다.'
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
					title: '사업별 업무',
					description: ''
				}
			},
			copy: koreanReportCopy,
			report: flowReportFixtureSnapshot,
			tasks: flowReportFixtureTasks,
			members: flowReportFixtureMembers,
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

		expect(sections.memberDistance.chartKind).toBe('memberScoreList');
		expect(sections.memberDistance.unit).toBe('점');
		expect(sections.memberDistance.total).toBe(279);
		expect(sections.memberDistance.averageValue).toBe(140);
		expect(sections.memberDistance.rows.map((row) => row.label)).toEqual(['김예시', '박예시', '장가칭', '정의', '최견본']);
		expect(sections.memberDistance.rows[0].total).toBe(flowReportFixtureMetrics.memberScoreDetails['member-kim'].currentScore);
		expect(sections.memberDistance.rows[0]).toMatchObject({ label: '김예시', total: 144, percent: 52 });
		expect(sections.memberDistance.rows[0].summary).toBe('주간 115점 · 월간 173점');
		expect(sections.memberDistance.rows[0].segments.map((segment) => [segment.label, segment.value])).toEqual([
			['점수', 144]
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
			['샘플거리', 5],
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
					memberDistance: { title: '구성원 점수', description: '' },
					weeklyDistanceTrend: { title: '주간 통계', description: '' },
					monthlyDistanceTrend: { title: '월간 통계', description: '' },
					businessDistance: { title: '사업별 업무', description: '' }
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

	test('keeps zero score members visible in member score rows', () => {
		const sections = buildFlowReportSections(flowReportFixtureMetrics, {
			emptyLabel: '이번 주 데이터 없음',
			sectionLabels: {
				weeklyStatus: { title: '주간 상태', description: '' },
				memberDistance: { title: '구성원 점수', description: '' },
				weeklyDistanceTrend: { title: '주간 통계', description: '' },
				monthlyDistanceTrend: { title: '월간 통계', description: '' },
				businessDistance: { title: '사업별 업무', description: '' }
			},
			copy: koreanReportCopy,
			tasks: flowReportFixtureTasks,
			members: flowReportFixtureMembers,
			definitions: flowReportFixtureDefinitions,
			weekStartISO: '2026-06-01'
		});

		expect(sections.memberDistance.rows.map((row) => [row.label, row.total, row.summary])).toEqual([
			['김예시', 144, '주간 115점 · 월간 173점'],
			['박예시', 135, '주간 155점 · 월간 115점'],
			['장가칭', 0, '주간 0점 · 월간 0점'],
			['정의', 0, '주간 0점 · 월간 0점'],
			['최견본', 0, '주간 0점 · 월간 0점']
		]);
	});

	test('excludes completed tasks without end dates from distance sections', () => {
		const sections = buildFlowReportSections(
			{
				totalTasks: 2,
				completedTasks: 2,
				requestedTasks: 0,
				pausedTasks: 0,
				stoppedTasks: 0,
				totalDistance: 3,
				statusCounts: { 완료: 2 },
				businessCounts: { 샘플거리: 2 },
				typeCounts: { 구현: 2 },
				memberDistances: { 김예시: 3 }
			},
			{
				emptyLabel: '이번 주 데이터 없음',
				sectionLabels: {
					weeklyStatus: { title: '주간 상태', description: '' },
					memberDistance: { title: '구성원 점수', description: '' },
					weeklyDistanceTrend: { title: '주간 통계', description: '' },
					monthlyDistanceTrend: { title: '월간 통계', description: '' },
					businessDistance: { title: '사업 거리', description: '' }
				},
				copy: koreanReportCopy,
				tasks: [
					{
						participantNames: ['김예시'],
						business: '샘플거리',
						type: '구현',
						size: 'M',
						status: '완료',
						startDate: '',
						endDate: '2026-06-02'
					},
					{
						participantNames: ['김예시'],
						business: '샘플거리',
						type: '구현',
						size: 'M',
						status: '완료',
						startDate: '',
						endDate: ''
					}
				],
				members: flowReportFixtureMembers,
				definitions: flowReportFixtureDefinitions,
				weekStartISO: '2026-06-01'
			}
		);

		expect(sections.weeklyStatus.total).toBe(3);
		expect(sections.weeklyStatus.rows.map((row) => row.total)).toEqual([0, 3, 0, 0, 0, 0, 0]);
		expect(sections.businessDistance.total).toBe(3);
	});

	test('reads legacy score metrics as distance metrics', () => {
		const { totalScore, memberScores, memberScoreDetails, ...legacyMetricBase } = flowReportFixtureMetrics;
		const legacyMetrics: FlowReportMetrics = {
			...legacyMetricBase,
			totalDistance: totalScore,
			memberDistances: memberScores
		};

		const sections = buildFlowReportSections(legacyMetrics, {
			emptyLabel: '이번 주 데이터 없음',
			sectionLabels: {
				weeklyStatus: { title: '주간 상태', description: '' },
				memberDistance: { title: '구성원 점수', description: '' },
				weeklyDistanceTrend: { title: '주간 통계', description: '' },
				monthlyDistanceTrend: { title: '월간 통계', description: '' },
				businessDistance: { title: '사업별 업무', description: '' }
			},
			copy: koreanReportCopy,
			tasks: flowReportFixtureTasks,
			members: flowReportFixtureMembers,
			definitions: flowReportFixtureDefinitions
		});

		expect(sections.memberDistance.total).toBe(279);
		expect(sections.memberDistance.averageValue).toBe(140);
		expect(sections.memberDistance.rows.map((row) => row.label)).toEqual(['김예시', '박예시', '장가칭', '정의', '최견본']);
	});

	test('localizes report system labels without translating user definitions', () => {
		const englishOptions = {
			emptyLabel: 'No data for this week',
			sectionLabels: {
				weeklyStatus: { title: 'Weekly daily type distance', description: '' },
				memberDistance: { title: 'Member score', description: '' },
				weeklyDistanceTrend: { title: 'Weekly statistics', description: '' },
				monthlyDistanceTrend: { title: 'Monthly statistics', description: '' },
				businessDistance: { title: 'Weekly business distance', description: '' }
			},
			copy: {
				weekdays: ['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun'],
				fallbackType: 'Other',
				fallbackBusiness: 'Unassigned',
				memberScoreLabel: 'Score',
				weeklyScoreLabel: 'Weekly',
				monthlyScoreLabel: 'Monthly',
				scoreUnit: ' pts',
				teamAverageLabel: 'Team average',
				memberScrollHint: '{count} members total · scroll inside list',
				currentWeekTrend: 'This week',
				previousWeekTrend: 'Last week',
				currentMonthTrend: 'This month',
				previousMonthTrend: 'Last month',
				monthlyDayLabelTemplate: 'Day {day}'
			},
			report: flowReportFixtureSnapshot,
			members: flowReportFixtureMembers,
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
		expect(sections.businessDistance.items.map((item) => item.label)).toEqual(['샘플거리', '김인턴', 'Unassigned']);
		expect(sections.memberDistance.rows[0].summary).toBe('Weekly 115 pts · Monthly 173 pts');
		expect(sections.memberDistance.teamAverageLabel).toBe('Team average');
		expect(sections.memberDistance.memberScrollHint).toBe('{count} members total · scroll inside list');
		expect(sections.weeklyDistanceTrend.trend.labels).toEqual(['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun']);
		expect(sections.weeklyDistanceTrend.trend.currentLabel).toBe('This week');
		expect(sections.weeklyDistanceTrend.trend.previousLabel).toBe('Last week');
		expect(sections.monthlyDistanceTrend.trend.currentLabel).toBe('This month');
		expect(sections.monthlyDistanceTrend.trend.previousLabel).toBe('Last month');
		expect(sections.monthlyDistanceTrend.trend.labelTemplate).toBe('Day {day}');
	});

	test('disambiguates duplicate member names when score metrics are keyed by member id', () => {
		const sections = buildFlowReportSections(
			{
				totalTasks: 0,
				completedTasks: 0,
				requestedTasks: 0,
				pausedTasks: 0,
				stoppedTasks: 0,
				totalDistance: 0,
				totalScore: 18,
				statusCounts: {},
				businessCounts: {},
				typeCounts: {},
				memberDistances: {},
				memberScoreDetails: {
					'member-a': { weeklyScore: 9, monthlyScore: 11, currentScore: 10 },
					'member-b': { weeklyScore: 7, monthlyScore: 9, currentScore: 8 }
				}
			},
			{
				emptyLabel: '이번 주 데이터 없음',
				sectionLabels: {
					weeklyStatus: { title: '주간 상태', description: '' },
					memberDistance: { title: '구성원 점수', description: '' },
					weeklyDistanceTrend: { title: '주간 통계', description: '' },
					monthlyDistanceTrend: { title: '월간 통계', description: '' },
					businessDistance: { title: '사업별 업무', description: '' }
				},
				copy: koreanReportCopy,
				members: [
					{ id: 'member-a', name: '김철수' },
					{ id: 'member-b', name: '김철수' }
				]
			}
		);

		expect(sections.memberDistance.rows.map((row) => row.label)).toEqual(['김철수 (member-a)', '김철수 (member-b)']);
	});
});
