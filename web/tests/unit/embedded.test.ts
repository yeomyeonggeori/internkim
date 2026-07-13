import { describe, expect, test } from 'bun:test';

import { isFrameEmbedded } from '../../src/lib/embedded';

describe('embedded frame detection', () => {
	test('identifies direct iframe embedding', () => {
		const topWindow = {};
		const frameWindow = { self: {}, top: topWindow };

		expect(isFrameEmbedded(frameWindow)).toBe(true);
	});

	test('identifies a top-level window', () => {
		const topWindow = {};
		const frameWindow = { self: topWindow, top: topWindow };

		expect(isFrameEmbedded(frameWindow)).toBe(false);
	});
});
