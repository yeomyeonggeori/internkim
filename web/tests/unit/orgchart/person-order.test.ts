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

	test('orders Korean names before English names on the same hire date', () => {
		const people = [
			{ userID: 'pptx', name: 'PPTX Tester', hireDate: '2026-04-01' },
			{ userID: 'gamyeong', name: '이샘플', hireDate: '2026-04-01' },
			{ userID: 'pyobon', name: '김표본', hireDate: '2026-04-01' },
			{ userID: 'aaron', name: 'Aaron', hireDate: '2026-04-01' }
		];

		expect([...people].sort(compareOrgchartPeople).map((person) => person.userID)).toEqual([
			'pyobon',
			'gamyeong',
			'aaron',
			'pptx'
		]);
	});

	test('orders Korean names before English names when hire dates are missing', () => {
		const people = [
			{ userID: 'pptx', name: 'PPTX Tester' },
			{ userID: 'gamyeong', name: '이샘플' },
			{ userID: 'pyobon', name: '김표본' },
			{ userID: 'aaron', name: 'Aaron' }
		];

		expect([...people].sort(compareOrgchartPeople).map((person) => person.userID)).toEqual([
			'pyobon',
			'gamyeong',
			'aaron',
			'pptx'
		]);
	});
});
