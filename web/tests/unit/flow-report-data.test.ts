import { describe, expect, test } from 'bun:test';
import { buildFlowReportSections } from '../../src/routes/flow/report/flow-report-data';
import { flowReportFixtureMetrics } from './fixtures/flow-report-summary';

const statusLabels: Record<string, string> = {
	완료: '완료',
	진행: '진행',
	요청: '요청',
	예정: '예정',
	일시정지: '일시정지',
	중단: '중단'
};

const statusDescriptions: Record<string, string> = {
	완료: '종료됨',
	진행: '작업 중',
	요청: '확인 대기',
	예정: '시작 전',
	일시정지: '재개 가능',
	중단: '종료 처리'
};

describe('buildFlowReportSections', () => {
	test('builds sorted report chart sections from flow metrics', () => {
		const sections = buildFlowReportSections(flowReportFixtureMetrics, {
			statusLabels,
			statusDescriptions,
			emptyLabel: '이번 주 데이터 없음',
			sectionLabels: {
				weeklyStatus: {
					title: '주간 상태',
					description: '상태별 업무 분포와 막힌 일을 빠르게 확인합니다.'
				},
				memberScores: {
					title: '구성원 거리',
					description: '완료와 진행 업무 기준의 주간 업무 거리입니다.'
				},
				businessDistance: {
					title: '대분류별 업무',
					description: ''
				},
				typeBreakdown: {
					title: '종류별 업무',
					description: ''
				}
			}
		});

		expect(sections.weeklyStatus.chartKind).toBe('stacked');
		expect(sections.weeklyStatus.total).toBe(14);
		expect(sections.weeklyStatus.alertValue).toBe(2);
		expect(sections.weeklyStatus.items.map((item) => item.label)).toEqual(['완료', '진행', '요청', '예정', '일시정지']);
		expect(sections.weeklyStatus.items[0]).toMatchObject({ value: 5, percent: 36, tone: 'success' });
		expect(sections.weeklyStatus.items[4]).toMatchObject({ value: 1, percent: 7, tone: 'paused', description: '재개 가능' });
		expect(sections.weeklyStatus.items.some((item) => item.label === '중단')).toBe(false);

		expect(sections.memberScores.chartKind).toBe('workload');
		expect(sections.memberScores.total).toBe(27);
		expect(sections.memberScores.averageValue).toBe(7);
		expect(sections.memberScores.items.map((item) => item.label)).toEqual(['김여명', '박예시', '최견본', '정의']);
		expect(sections.memberScores.items[0]).toMatchObject({ value: 11, percent: 41, tone: 'member' });

		expect(sections.businessDistance.chartKind).toBe('donut');
		expect(sections.businessDistance.items.map((item) => item.label)).toEqual(['개발', '운영', '기획']);
		expect(sections.businessDistance.items[0]).toMatchObject({ tone: 'business' });
		expect(sections.typeBreakdown.chartKind).toBe('treemap');
		expect(sections.typeBreakdown.items.map((item) => item.label)).toEqual(['구현', '회의', '검증', '문서']);
		expect(sections.typeBreakdown.items[0]).toMatchObject({ tone: 'type' });
	});

	test('keeps empty report sections explicit when metrics have no values', () => {
		const sections = buildFlowReportSections(
			{
				totalTasks: 0,
				completedTasks: 0,
				requestedTasks: 0,
				pausedTasks: 0,
				stoppedTasks: 0,
				totalScore: 0,
				statusCounts: {},
				businessCounts: {},
				typeCounts: {},
				memberScores: {}
			},
			{
				statusLabels,
				statusDescriptions,
				emptyLabel: '이번 주 데이터 없음',
				sectionLabels: {
					weeklyStatus: { title: '주간 상태', description: '' },
					memberScores: { title: '구성원 거리', description: '' },
					businessDistance: { title: '대분류별 업무', description: '' },
					typeBreakdown: { title: '종류별 업무', description: '' }
				}
			}
		);

		expect(sections.weeklyStatus.items).toEqual([]);
		expect(sections.weeklyStatus.emptyLabel).toBe('이번 주 데이터 없음');
		expect(sections.memberScores.total).toBe(0);
		expect(sections.businessDistance.maxValue).toBe(1);
		expect(sections.typeBreakdown.maxValue).toBe(1);
	});
});
