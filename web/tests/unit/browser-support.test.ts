import { describe, expect, test } from 'bun:test';
import { isBrowserOutdated, type BrowserScope } from '../../src/lib/browser-support';

function currentBrowser(): BrowserScope {
	return {
		AbortSignal: { timeout() {} },
		Array: { prototype: { toSorted() {}, toReversed() {}, findLast() {} } },
		Map: { groupBy() {} },
		URL: { canParse() {} },
		structuredClone() {}
	};
}

describe('a browser the app can draw every screen in', () => {
	test('is the one running these tests', () => {
		expect(isBrowserOutdated()).toBe(false);
	});

	test('has every built-in the app calls', () => {
		expect(isBrowserOutdated(currentBrowser())).toBe(false);
	});
});

describe('an outdated browser', () => {
	test('is one without sorted copies of a list, as Chrome 109 is', () => {
		const chrome109 = { ...currentBrowser(), Array: { prototype: { findLast() {} } } };
		expect(isBrowserOutdated(chrome109)).toBe(true);
	});

	test('is one without grouping into a map, as Safari 16 is', () => {
		const safari16 = { ...currentBrowser(), Map: {} };
		expect(isBrowserOutdated(safari16)).toBe(true);
	});
});
