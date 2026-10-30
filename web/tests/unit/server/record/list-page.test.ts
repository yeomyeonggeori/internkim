import { describe, expect, test } from 'bun:test';
import { ascendingPositions, descendingPositions, pageOf } from '../../../../src/lib/server/public-api/record/list-page';

type Row = { id: string; startsAt: string };

const rows: Row[] = [
	{ id: 'c', startsAt: '2026-09-02' },
	{ id: 'a', startsAt: '2026-09-01' },
	{ id: 'e', startsAt: '2026-09-03' },
	{ id: 'b', startsAt: '2026-09-01' },
	{ id: 'd', startsAt: '2026-09-02' }
];

const byStart = { positionOf: (row: Row) => [row.startsAt, row.id], compare: ascendingPositions };

function everyPage(order: typeof byStart, limit: number) {
	const pages = [];
	let cursor: string | undefined;
	do {
		const page = pageOf('event_list', rows, order, { limit, ...(cursor ? { cursor } : {}) });
		pages.push(page);
		cursor = page.nextCursor;
	} while (cursor);
	return pages;
}

describe('a list read a page at a time', () => {
	test('walks every row once, in order, ties broken by ID', () => {
		const pages = everyPage(byStart, 2);
		expect(pages.map((page) => page.items.map((row) => row.id))).toEqual([['a', 'b'], ['c', 'd'], ['e']]);
		expect(pages.every((page) => page.totalCount === rows.length)).toBe(true);
	});

	test('says no page is left on the last one, and on a page that holds everything', () => {
		expect(everyPage(byStart, 2).at(-1)?.nextCursor).toBeUndefined();
		expect(pageOf('event_list', rows, byStart, { limit: 5 }).nextCursor).toBeUndefined();
	});

	test('answers fifty when the call names no limit', () => {
		const many = Array.from({ length: 60 }, (_, index) => ({ id: String(index).padStart(2, '0'), startsAt: '2026-09-01' }));
		const page = pageOf('event_list', many, byStart, {});
		expect(page.items.length).toBe(50);
		expect(page.totalCount).toBe(60);
		expect(page.nextCursor).toBeDefined();
	});

	test('pages newest first when the order descends', () => {
		const newestFirst = { positionOf: byStart.positionOf, compare: descendingPositions };
		expect(everyPage(newestFirst, 2).flatMap((page) => page.items.map((row) => row.id))).toEqual(['e', 'd', 'c', 'b', 'a']);
	});

	test('refuses a cursor no result gave, naming the tool', () => {
		for (const cursor of ['not-base64!', btoa('{"name":1}'), btoa('[]'), btoa('[1,2]')]) {
			expect(() => pageOf('leave_list', rows, byStart, { cursor })).toThrow('cursor is not one a leave_list result gave');
		}
	});
});
