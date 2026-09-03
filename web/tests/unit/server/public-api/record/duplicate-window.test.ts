import { describe, expect, test } from 'bun:test';

import { addedWithinTheDuplicateWindow } from '../../../../../src/lib/server/public-api/record/task-tools';

const now = new Date('2026-09-04T01:00:00.000Z');

describe('the duplicate window', () => {
	test('takes a row the record stamped a moment ahead of the request clock', () => {
		expect(addedWithinTheDuplicateWindow('2026-09-04T01:00:01.500Z', now)).toBe(true);
	});

	test('takes a row added a minute ago', () => {
		expect(addedWithinTheDuplicateWindow('2026-09-04T00:59:00.000Z', now)).toBe(true);
	});

	test('leaves a row added eleven minutes ago', () => {
		expect(addedWithinTheDuplicateWindow('2026-09-04T00:49:00.000Z', now)).toBe(false);
	});
});
