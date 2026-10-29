import { describe, expect, test } from 'bun:test';
import { leavesTheApp } from '$lib/native-shell/external-links';

const appOrigin = 'https://intern.kim';

describe('leavesTheApp', () => {
	test('keeps a link to the app itself inside the app', () => {
		expect(leavesTheApp({ protocol: 'https:', origin: appOrigin }, appOrigin)).toBe(false);
	});

	test('sends a link to another site to the browser', () => {
		expect(leavesTheApp({ protocol: 'https:', origin: 'https://map.example.com' }, appOrigin)).toBe(true);
		expect(leavesTheApp({ protocol: 'http:', origin: 'http://example.com' }, appOrigin)).toBe(true);
	});

	test('hands mail and phone links to their own apps', () => {
		expect(leavesTheApp({ protocol: 'mailto:', origin: 'null' }, appOrigin)).toBe(true);
		expect(leavesTheApp({ protocol: 'tel:', origin: 'null' }, appOrigin)).toBe(true);
	});

	test('leaves blob and data links to the page', () => {
		expect(leavesTheApp({ protocol: 'blob:', origin: appOrigin }, appOrigin)).toBe(false);
		expect(leavesTheApp({ protocol: 'data:', origin: 'null' }, appOrigin)).toBe(false);
	});
});
