import { describe, expect, test } from 'bun:test';
import { swipeReplyLimitPixels, swipeReplyThresholdPixels, swipeStateOf } from '$lib/components/channel/swipe-to-reply';

describe('swipeStateOf', () => {
	test('a short left drag is unarmed with the offset matching the drag distance', () => {
		const state = swipeStateOf(-20, 0);
		expect(state.isHorizontal).toBe(true);
		expect(state.offsetPixels).toBe(20);
		expect(state.isArmed).toBe(false);
	});

	test('a long left drag is armed and clamped to the limit', () => {
		const state = swipeStateOf(-500, 0);
		expect(state.isHorizontal).toBe(true);
		expect(state.offsetPixels).toBe(swipeReplyLimitPixels);
		expect(state.isArmed).toBe(true);
		expect(state.offsetPixels).toBeGreaterThanOrEqual(swipeReplyThresholdPixels);
	});

	test('a right drag moves nothing', () => {
		const state = swipeStateOf(50, 0);
		expect(state.offsetPixels).toBe(0);
		expect(state.isArmed).toBe(false);
	});

	test('a mostly vertical drag moves nothing even when far', () => {
		const state = swipeStateOf(-10, 300);
		expect(state.isHorizontal).toBe(false);
		expect(state.offsetPixels).toBe(0);
		expect(state.isArmed).toBe(false);
	});
});
