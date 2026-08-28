import { describe, expect, test } from 'bun:test';
import { mergeChangedTasks, newestStamp } from '../../../src/lib/task/task-cache';

type Row = { id: string; updated_at: string; title: string };

const cached: Row[] = [
	{ id: 'a', updated_at: '2026-08-01T00:00:00Z', title: '첫 일' },
	{ id: 'b', updated_at: '2026-08-02T00:00:00Z', title: '둘째 일' }
];

describe('mergeChangedTasks', () => {
	test('a changed task replaces the one held', () => {
		const changed: Row[] = [{ id: 'b', updated_at: '2026-08-03T00:00:00Z', title: '고친 둘째 일' }];
		const merged = mergeChangedTasks(cached, changed, new Set(['a', 'b']));
		expect(merged.find((task) => task.id === 'b')?.title).toBe('고친 둘째 일');
		expect(merged).toHaveLength(2);
	});

	test('a new task joins the ones held', () => {
		const changed: Row[] = [{ id: 'c', updated_at: '2026-08-03T00:00:00Z', title: '새 일' }];
		const merged = mergeChangedTasks(cached, changed, new Set(['a', 'b', 'c']));
		expect(merged.map((task) => task.id).sort()).toEqual(['a', 'b', 'c']);
	});

	test('a task that no longer exists is dropped, which is what updated_at alone would miss', () => {
		const merged = mergeChangedTasks(cached, [], new Set(['a']));
		expect(merged.map((task) => task.id)).toEqual(['a']);
	});
});

describe('newestStamp', () => {
	test('is the latest change among the tasks, so the next ask starts there', () => {
		expect(newestStamp(cached)).toBe('2026-08-02T00:00:00Z');
	});

	test('with nothing held it asks for everything rather than skipping', () => {
		expect(newestStamp([])).toBe('');
	});
});
