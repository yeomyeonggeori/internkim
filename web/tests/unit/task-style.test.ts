import { describe, expect, test } from 'bun:test';
import {
	relationshipStatusIconClass,
	statusIconClass
} from '../../src/routes/task/task-style';

describe('flow status icon styles', () => {
	const cases: Array<[string, string]> = [
		['completed', 'text-[#16a34a]'],
		['in_progress', 'text-[#0284c7]'],
		['planned', 'text-[#d97706]'],
		['requested', 'text-[#7c3aed]'],
		['paused', 'text-[#e11d48]'],
		['rejected', 'text-[#dc2626]'],
		['stopped', 'text-[#dc2626]']
	];

	for (const [status, expected] of cases) {
		test(`uses the existing palette for ${status}`, () => {
			expect(statusIconClass(status)).toBe(expected);
		});
	}

	test('uses the muted icon color for an unknown status', () => {
		expect(statusIconClass('알 수 없음')).toBe('text-muted-foreground');
	});
});

describe('flow relationship status icon styles', () => {
	test('uses completed styling only for completed work', () => {
		expect(relationshipStatusIconClass('completed')).toBe('text-[#16a34a]');
	});

	test('uses one incomplete style for active statuses', () => {
		for (const status of ['requested', 'planned', 'in_progress', 'paused']) {
			expect(relationshipStatusIconClass(status)).toBe('text-[#7c3aed]');
		}
	});

	test('uses the incomplete style for statuses excluded from progress', () => {
		for (const status of ['rejected', 'stopped']) {
			expect(relationshipStatusIconClass(status)).toBe('text-[#7c3aed]');
		}
	});
});
