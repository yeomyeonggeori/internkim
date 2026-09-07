import { describe, expect, test } from 'bun:test';
import {
	nextSortState,
	sortRows,
	type CRMSortComparators,
	type CRMSortState
} from '../../../src/routes/crm/crm-table-sort';

type Row = {
	name: string;
	amount?: number;
	due: string;
};

const rows: Row[] = [
	{ name: '샘플 임팩트 랩', amount: 5000, due: '2026-08-02' },
	{ name: '가나다 재단', amount: 12000, due: '' },
	{ name: '예시 벤처스', amount: undefined, due: '2026-07-01' }
];

const comparators: CRMSortComparators<Row> = {
	name: (row) => row.name,
	amount: (row) => row.amount,
	due: (row) => row.due
};

function namesSortedBy(key: string, direction: CRMSortState['direction']): string[] {
	return sortRows(rows, { key, direction }, comparators).map((row) => row.name);
}

describe('CRM table sorting', () => {
	test('cycles a column through ascending, descending and unsorted', () => {
		const ascending = nextSortState(null, 'name');
		expect(ascending).toEqual({ key: 'name', direction: 'ascending' });
		const descending = nextSortState(ascending, 'name');
		expect(descending).toEqual({ key: 'name', direction: 'descending' });
		expect(nextSortState(descending, 'name')).toBe(null);
	});

	test('starts a different column ascending rather than continuing the cycle', () => {
		expect(nextSortState({ key: 'name', direction: 'descending' }, 'amount')).toEqual({
			key: 'amount',
			direction: 'ascending'
		});
	});

	test('orders Korean names the way a Korean reader expects', () => {
		expect(namesSortedBy('name', 'ascending')).toEqual(['가나다 재단', '샘플 임팩트 랩', '예시 벤처스']);
		expect(namesSortedBy('name', 'descending')).toEqual(['예시 벤처스', '샘플 임팩트 랩', '가나다 재단']);
	});

	test('compares numbers by value, not by their text', () => {
		expect(sortRows(rows, { key: 'amount', direction: 'ascending' }, comparators).map((row) => row.amount)).toEqual([
			5000,
			12000,
			undefined
		]);
	});

	test('leaves empty values last in both directions', () => {
		expect(namesSortedBy('amount', 'ascending').at(-1)).toBe('예시 벤처스');
		expect(namesSortedBy('amount', 'descending').at(-1)).toBe('예시 벤처스');
		expect(namesSortedBy('due', 'ascending').at(-1)).toBe('가나다 재단');
		expect(namesSortedBy('due', 'descending').at(-1)).toBe('가나다 재단');
	});

	test('returns the rows untouched when nothing is sorted or the key is unknown', () => {
		expect(sortRows(rows, null, comparators)).toBe(rows);
		expect(sortRows(rows, { key: 'missing', direction: 'ascending' }, comparators)).toBe(rows);
	});

	test('does not mutate the rows it was given', () => {
		const original = [...rows];
		sortRows(rows, { key: 'name', direction: 'descending' }, comparators);
		expect(rows).toEqual(original);
	});
});

describe('a default sort with a second key', () => {
	type Ranked = { name: string; rank: number; date: string };

	const ranked: Ranked[] = [
		{ name: 'older-open', rank: 2, date: '2026-09-01' },
		{ name: 'newer-open', rank: 2, date: '2026-09-05' },
		{ name: 'newer-done', rank: 0, date: '2026-09-09' }
	];

	const rankComparators = { rank: (row: Ranked) => row.rank };

	test('breaks a tie on the second key without disturbing the first', () => {
		const sorted = sortRows(ranked, { key: 'rank', direction: 'descending' }, rankComparators, {
			read: (row: Ranked) => row.date,
			direction: 'descending'
		});

		expect(sorted.map((row) => row.name)).toEqual(['newer-open', 'older-open', 'newer-done']);
	});

	test('leaves the order alone when no tie-breaker is given', () => {
		const sorted = sortRows(ranked, { key: 'rank', direction: 'descending' }, rankComparators);

		expect(sorted.map((row) => row.rank)).toEqual([2, 2, 0]);
	});
});
