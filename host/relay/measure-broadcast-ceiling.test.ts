import { describe, expect, test } from 'bun:test';
import { jsonBodyOfExactly } from './measure-broadcast-ceiling';

describe('jsonBodyOfExactly', () => {
	test('a probe body is the size it claims, or the measurement is off by that much', () => {
		for (const bytes of [16, 1_024, 3_000_000]) {
			expect(new TextEncoder().encode(jsonBodyOfExactly(bytes)).length).toBe(bytes);
		}
	});

	test('a size too small to hold the surrounding json is refused, never silently padded', () => {
		expect(() => jsonBodyOfExactly(4)).toThrow('a json body cannot be 4 bytes');
	});
});
