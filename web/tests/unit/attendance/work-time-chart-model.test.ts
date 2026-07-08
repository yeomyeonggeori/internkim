// 근무 시간 차트 모델의 날짜 라벨 계산을 검증한다.
import { describe, expect, test } from 'bun:test';
import {
	buildSeries,
	chartPointTotalMinutes,
} from '../../../src/routes/attendance/shared/work-time-chart-model';

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
				{ date: '2026-05-31', minutesByLocation: { office: 2 } },
				{ date: '2026-06-01', minutesByLocation: { office: 1, remote: 2 } },
				{ date: '2026-06-30', minutesByLocation: { remote: 5 } },
				{ date: '2026-07-04', minutesByLocation: { office: 7 } },
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
		expect(englishSeries.map((point) => point.values)).toEqual([
			{ office: 3, remote: 2 },
			{},
			{},
			{},
			{ remote: 5, office: 7 },
		]);
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
				{ date: '2026-01-12', minutesByLocation: { 사무실: 2 } },
				{ date: '2026-06-24', minutesByLocation: { 사무실: 3, 재택: 2 } },
				{ date: '2026-12-31', minutesByLocation: { 재택: 7 } },
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
		expect(koreanSeries.map((point) => point.values)).toEqual([
			{ 사무실: 2 },
			{},
			{},
			{},
			{},
			{ 사무실: 3, 재택: 2 },
			{},
			{},
			{},
			{},
			{},
			{ 재택: 7 },
		]);
	});

	const labels = {
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
	};

	test('keeps per location minutes on daily points and derives the total', () => {
		const series = buildSeries(
			'2026-06',
			[{ date: '2026-06-02', minutesByLocation: { office: 90, remote: 30 } }],
			'day',
			labels
		);
		const point = series.find((seriesPoint) => seriesPoint.key === '2026-06-02');

		expect(point?.values).toEqual({ office: 90, remote: 30 });
		expect(point ? chartPointTotalMinutes(point) : 0).toBe(120);
		expect(series.find((seriesPoint) => seriesPoint.key === '2026-06-03')?.values).toEqual({});
	});

	test('trims future days when the viewed month is the current month', () => {
		const series = buildSeries('2026-06', [], 'day', labels, { today: '2026-06-08' });

		expect(series.map((point) => point.key)).toEqual([
			'2026-06-01',
			'2026-06-02',
			'2026-06-03',
			'2026-06-04',
			'2026-06-05',
			'2026-06-06',
			'2026-06-07',
			'2026-06-08',
		]);
	});

	test('keeps all days for a fully past month', () => {
		const series = buildSeries('2026-05', [], 'day', labels, { today: '2026-06-08' });

		expect(series.length).toBe(31);
		expect(series[series.length - 1].key).toBe('2026-05-31');
	});

	test('keeps all days for a fully future month instead of returning an empty series', () => {
		const series = buildSeries('2026-07', [], 'day', labels, { today: '2026-06-08' });

		expect(series.length).toBe(31);
		expect(series[0].key).toBe('2026-07-01');
		expect(series[series.length - 1].key).toBe('2026-07-31');
	});

	test('trims future weeks whose start date is after today', () => {
		const series = buildSeries('2026-06', [], 'week', labels, { today: '2026-06-08' });

		expect(series.map((point) => point.label)).toEqual(['W1', 'W2']);
	});

	test('keeps all weeks for a fully future month', () => {
		const series = buildSeries('2026-07', [], 'week', labels, { today: '2026-06-08' });

		expect(series.map((point) => point.label)).toEqual(['W1', 'W2', 'W3', 'W4', 'W5']);
	});

	test('trims months after the current month when the viewed year is the current year', () => {
		const series = buildSeries('2026-06', [], 'month', labels, { today: '2026-06-08' });

		expect(series.map((point) => point.label)).toEqual(['M1', 'M2', 'M3', 'M4', 'M5', 'M6']);
	});

	test('keeps all twelve months for a fully future year', () => {
		const series = buildSeries('2027-06', [], 'month', labels, { today: '2026-06-08' });

		expect(series.length).toBe(12);
	});

	test('keeps all twelve months for a fully past year', () => {
		const series = buildSeries('2025-06', [], 'month', labels, { today: '2026-06-08' });

		expect(series.length).toBe(12);
	});
});
