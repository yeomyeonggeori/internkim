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
	test('a widget holding nothing is handed a key', () => {
		expect(widgetNeedsToken({ installID: 'abc', tokenName: '' }, [])).toBe(true);
	});

	test('a key the member has revoked is replaced', () => {
		expect(
			widgetNeedsToken({ installID: 'abc', tokenName: 'ios-widget-abc' }, [{ name: 'pat-1' }])
		).toBe(true);
	});

	test('a key the member still holds is left alone', () => {
		expect(
			widgetNeedsToken({ installID: 'abc', tokenName: 'ios-widget-abc' }, [
				{ name: 'pat-1' },
				{ name: 'ios-widget-abc' }
			])
		).toBe(false);
	});
});
