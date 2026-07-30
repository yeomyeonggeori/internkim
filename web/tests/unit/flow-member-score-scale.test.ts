import { describe, expect, test } from 'bun:test';
import { memberScoreScale, memberScoreWidth } from '../../src/routes/flow/report/flow-member-score-scale';

describe('member score scale', () => {
	test('spreads close scores across the bar instead of showing near-full bars', () => {
		const scale = memberScoreScale([99, 101, 103]);

		expect(memberScoreWidth(103, scale)).toBe(100);
		expect(memberScoreWidth(101, scale)).toBe(59);
		expect(memberScoreWidth(99, scale)).toBe(18);
	});

	test('ignores members without a score so the scored range stays readable', () => {
		const scale = memberScoreScale([0, 0, 99, 103]);

		expect(memberScoreWidth(99, scale)).toBe(18);
		expect(memberScoreWidth(0, scale)).toBe(4);
	});

	test('fills the bar when every score matches', () => {
		expect(memberScoreWidth(80, memberScoreScale([80, 80]))).toBe(100);
	});
});
