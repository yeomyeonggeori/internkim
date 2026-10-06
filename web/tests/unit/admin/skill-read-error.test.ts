import { describe, expect, test } from 'bun:test';
import { isSkillReadAccessDenied, SkillReadError } from '../../../src/routes/admin/skill-read-error';

describe('skill read authorization errors', () => {
	for (const status of [401, 403]) {
		test(`identifies a typed ${status} response so cached records can be cleared`, () => {
			const error = new SkillReadError('Skill request refused', status);
			expect(error).toBeInstanceOf(Error);
			expect(error.status).toBe(status);
			expect(error.message).toBe('Skill request refused');
			expect(isSkillReadAccessDenied(error)).toBe(true);
		});
	}

	test('keeps cached records on a typed temporary 503 failure', () => {
		const error = new SkillReadError('Skill request unavailable', 503);
		expect(error.status).toBe(503);
		expect(isSkillReadAccessDenied(error)).toBe(false);
	});

	test('does not infer authorization denial from error text or an untyped object', () => {
		for (const error of [new Error('Skill inventory request returned 403'), { status: 401 }, '403', null, undefined]) {
			expect(isSkillReadAccessDenied(error)).toBe(false);
		}
	});

	test('does not treat other typed HTTP failures as lost authority', () => {
		for (const status of [400, 404, 408, 429, 500, 502, 504]) {
			expect(isSkillReadAccessDenied(new SkillReadError('Skill read failed', status))).toBe(false);
		}
	});
});
