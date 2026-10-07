import { describe, expect, test } from 'bun:test';
import { rangeAfterClick } from '../../src/lib/components/channel/message-range';

describe('rangeAfterClick', () => {
	test('the first click makes that one message the range', () => {
		expect(rangeAfterClick(null, 4)).toEqual({ start: 4, end: 4 });
	});

	test('a click above the range moves its start there', () => {
		expect(rangeAfterClick({ start: 4, end: 6 }, 1)).toEqual({ start: 1, end: 6 });
	});

	test('a click below the range moves its end there', () => {
		expect(rangeAfterClick({ start: 4, end: 6 }, 9)).toEqual({ start: 4, end: 9 });
	});

	test('a click inside the range pulls in the nearer edge', () => {
		expect(rangeAfterClick({ start: 2, end: 10 }, 3)).toEqual({ start: 3, end: 10 });
		expect(rangeAfterClick({ start: 2, end: 10 }, 8)).toEqual({ start: 2, end: 8 });
	});

	test('a click as far from both edges pulls in the end', () => {
		expect(rangeAfterClick({ start: 2, end: 10 }, 6)).toEqual({ start: 2, end: 6 });
	});

	test('a click on a one-message range keeps it', () => {
		expect(rangeAfterClick({ start: 5, end: 5 }, 5)).toEqual({ start: 5, end: 5 });
	});
});

