import { describe, expect, test } from 'bun:test';
import { z } from 'zod';

import { readWithNullAsAbsent } from '../../../src/lib/server/public-api/null-as-absent';

const answer = z.strictObject({
	status: z.string(),
	eventID: z.string().nullable(),
	note: z.string().optional(),
	attendance: z.array(z.strictObject({ location: z.string().nullable(), memo: z.string().optional() }))
});

describe('null is read as a field left out', () => {
	test('a field that may be null reads the same left out or null', () => {
		for (const written of [{ status: 'asked', attendance: [] }, { status: 'asked', eventID: null, attendance: [] }]) {
			expect(answer.safeParse(readWithNullAsAbsent(answer, written)).success).toBe(true);
		}
	});

	test('a field that may be left out reads the same null or left out, at any depth', () => {
		const read = readWithNullAsAbsent(answer, {
			status: 'added',
			eventID: 'event-1',
			note: null,
			attendance: [{ memo: null }]
		});
		expect(read).toEqual({ status: 'added', eventID: 'event-1', attendance: [{ location: null }] });
		expect(answer.safeParse(read).success).toBe(true);
	});

	test('a field that must be given is missing when it is null', () => {
		const parsed = answer.safeParse(readWithNullAsAbsent(answer, { status: null, attendance: [] }));
		expect(parsed.success).toBe(false);
	});
});
