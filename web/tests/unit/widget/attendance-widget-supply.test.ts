import { describe, expect, test } from 'bun:test';
import {
	widgetNeedsToken,
	widgetTokenNameFor
} from '$lib/widget/attendance-widget-supply';

describe('widgetTokenNameFor', () => {
	test('names the token after the install so two phones never collide', () => {
		expect(widgetTokenNameFor('8E1F2C3D-4A5B-6C7D-8E9F-0A1B2C3D4E5F')).toBe('ios-widget-8e1f2c3d');
	});

	test('keeps the name within what the token store takes', () => {
		expect(widgetTokenNameFor('8E1F2C3D-4A5B-6C7D-8E9F-0A1B2C3D4E5F').length).toBeLessThanOrEqual(64);
	});
});

describe('widgetNeedsToken', () => {
	const now = new Date('2026-10-01T00:00:00.000Z');
	const farAway = '2026-12-30T00:00:00.000Z';

	test('a widget holding nothing is handed a key', () => {
		expect(widgetNeedsToken({ installID: 'abc', tokenName: '' }, [], now)).toBe(true);
	});

	test('a key the member has revoked is replaced', () => {
		expect(
			widgetNeedsToken({ installID: 'abc', tokenName: 'ios-widget-abc' }, [{ name: 'pat-1', expiresAt: farAway }], now)
		).toBe(true);
	});

	test('a key the member still holds is left alone', () => {
		expect(
			widgetNeedsToken(
				{ installID: 'abc', tokenName: 'ios-widget-abc' },
				[
					{ name: 'pat-1', expiresAt: farAway },
					{ name: 'ios-widget-abc', expiresAt: farAway }
				],
				now
			)
		).toBe(false);
	});

	test('a key within a month of expiring is renewed', () => {
		expect(
			widgetNeedsToken(
				{ installID: 'abc', tokenName: 'ios-widget-abc' },
				[{ name: 'ios-widget-abc', expiresAt: '2026-10-20T00:00:00.000Z' }],
				now
			)
		).toBe(true);
	});
});
