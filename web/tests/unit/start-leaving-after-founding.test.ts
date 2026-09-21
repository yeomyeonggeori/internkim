import { describe, expect, test } from 'bun:test';
import { leavesRightAfterFounding } from '../../src/routes/start/leaving-after-founding';

describe('leaving the founding screen on its own', () => {
	test('a founder who invited nobody goes straight back', () => {
		expect(leavesRightAfterFounding('/oauth/consent?authorization_id=abc', 0)).toBe(true);
	});

	test('a founder holding temporary passwords stays until they say so', () => {
		expect(leavesRightAfterFounding('/oauth/consent?authorization_id=abc', 1)).toBe(false);
		expect(leavesRightAfterFounding('/oauth/consent?authorization_id=abc', 4)).toBe(false);
	});

	test('with nowhere to go back to, the card stands either way', () => {
		expect(leavesRightAfterFounding('', 0)).toBe(false);
		expect(leavesRightAfterFounding('', 3)).toBe(false);
	});
});
