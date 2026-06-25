// 근무 시간 차트 모델의 날짜 라벨 계산을 검증한다.
import { describe, expect, test } from 'bun:test';
import { buildSeries } from '../../../src/routes/attendance/shared/work-time-chart-model';

describe('work time chart model', () => {
	test('adds localized weekday labels to daily tooltip labels', () => {
		const koreanSeries = buildSeries('2026-06', [], 'day', {
			weekLabelTemplate: '{index}주',
			monthLabelTemplate: '{index}월',
			total: '전체',
			weekdaySunday: '일',
			weekdayMonday: '월',
			weekdayTuesday: '화',
			weekdayWednesday: '수',
			weekdayThursday: '목',
			weekdayFriday: '금',
			weekdaySaturday: '토',
		});
		const englishSeries = buildSeries('2026-06', [], 'day', {
			weekLabelTemplate: 'Week {index}',
			monthLabelTemplate: 'M{index}',
			total: 'Total',
			weekdaySunday: 'Sun',
			weekdayMonday: 'Mon',
			weekdayTuesday: 'Tue',
			weekdayWednesday: 'Wed',
			weekdayThursday: 'Thu',
			weekdayFriday: 'Fri',
			weekdaySaturday: 'Sat',
		});

		expect(koreanSeries.find((point) => point.key === '2026-06-07')?.tooltipLabel).toBe('6/7 일');
		expect(englishSeries.find((point) => point.key === '2026-06-07')?.tooltipLabel).toBe('6/7 Sun');
	});

	test('uses compact English weekly labels', () => {
		const englishSeries = buildSeries('2026-06', [], 'week', {
			weekLabelTemplate: 'W{index}',
			monthLabelTemplate: 'M{index}',
			total: 'Total',
			weekdaySunday: 'Sun',
			weekdayMonday: 'Mon',
			weekdayTuesday: 'Tue',
			weekdayWednesday: 'Wed',
			weekdayThursday: 'Thu',
			weekdayFriday: 'Fri',
			weekdaySaturday: 'Sat',
		});

		expect(englishSeries.map((point) => point.label)).toEqual(['W1', 'W2', 'W3', 'W4', 'W5']);
		expect(englishSeries.map((point) => point.tooltipLabel)).toEqual(['W1', 'W2', 'W3', 'W4', 'W5']);
	});

	test('includes adjacent month dates in fixed Sunday to Saturday weekly buckets', () => {
		const englishSeries = buildSeries(
			'2026-06',
			[
				{ date: '2026-05-31', value: 2 },
				{ date: '2026-06-01', value: 3 },
				{ date: '2026-06-30', value: 5 },
				{ date: '2026-07-04', value: 7 },
			],
			'week',
			{
				weekLabelTemplate: 'W{index}',
				monthLabelTemplate: 'M{index}',
				total: 'Total',
				weekdaySunday: 'Sun',
				weekdayMonday: 'Mon',
				weekdayTuesday: 'Tue',
				weekdayWednesday: 'Wed',
				weekdayThursday: 'Thu',
				weekdayFriday: 'Fri',
				weekdaySaturday: 'Sat',
			}
		);

		expect(englishSeries.map((point) => point.label)).toEqual(['W1', 'W2', 'W3', 'W4', 'W5']);
		expect(englishSeries.map((point) => point.value)).toEqual([5, 0, 0, 0, 12]);
	});

	test('uses four weekly buckets for a 28 day month', () => {
		const englishSeries = buildSeries('2026-02', [], 'week', {
			weekLabelTemplate: 'W{index}',
			monthLabelTemplate: 'M{index}',
			total: 'Total',
			weekdaySunday: 'Sun',
			weekdayMonday: 'Mon',
			weekdayTuesday: 'Tue',
			weekdayWednesday: 'Wed',
			weekdayThursday: 'Thu',
			weekdayFriday: 'Fri',
			weekdaySaturday: 'Sat',
		});

		expect(englishSeries.map((point) => point.label)).toEqual(['W1', 'W2', 'W3', 'W4']);
	});

	test('uses twelve monthly buckets across the selected year', () => {
		const koreanSeries = buildSeries(
			'2026-06',
			[
				{ date: '2026-01-12', value: 2 },
				{ date: '2026-06-24', value: 5 },
				{ date: '2026-12-31', value: 7 },
			],
			'month',
			{
				weekLabelTemplate: '{index}주',
				monthLabelTemplate: '{index}월',
				total: '합계',
				weekdaySunday: '일',
				weekdayMonday: '월',
				weekdayTuesday: '화',
				weekdayWednesday: '수',
				weekdayThursday: '목',
				weekdayFriday: '금',
				weekdaySaturday: '토',
			}
		);

		expect(koreanSeries.map((point) => point.label)).toEqual([
			'1월',
			'2월',
			'3월',
			'4월',
			'5월',
			'6월',
			'7월',
			'8월',
			'9월',
			'10월',
			'11월',
			'12월',
		]);
		expect(koreanSeries.map((point) => point.value)).toEqual([2, 0, 0, 0, 0, 5, 0, 0, 0, 0, 0, 7]);
	});
});
