import { describe, expect, test } from 'bun:test';
import { compareOrgchartPeople } from '../../../src/lib/orgchart/person-order';

describe('orgchart person ordering', () => {
	test('orders by hire date before name and puts missing dates after dated people', () => {
		const people = [
			{ userID: 'missing-zara', name: 'Zara' },
			{ userID: 'newer-aaron', name: 'Aaron', hireDate: '2026-04-01' },
			{ userID: 'older-min', name: 'Min', hireDate: '2026-02-01' },
			{ userID: 'missing-ada', name: 'Ada' },
			{ userID: 'newer-bora', name: 'Bora', hireDate: '2026-04-01' }
		];

		expect([...people].sort(compareOrgchartPeople).map((person) => person.userID)).toEqual([
			'older-min',
			'newer-aaron',
			'newer-bora',
			'missing-ada',
			'missing-zara'
		]);
	});
});
