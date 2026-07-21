import { describe, expect, test } from 'bun:test';
import { compareOrgchartPeople, orderOrgchartPeopleByHierarchy } from '../../../src/lib/orgchart/person-order';

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

	test('places each supervisor before recursively ordered direct reports', () => {
		const people = [
			{ userID: 'junior', name: 'Junior', hireDate: '2026-03-01', supervisorID: 'manager' },
			{ userID: 'manager', name: 'Manager', hireDate: '2026-04-01', supervisorID: 'leader' },
			{ userID: 'leader', name: 'Leader', hireDate: '2026-05-01' },
			{ userID: 'peer', name: 'Peer', hireDate: '2026-01-01', supervisorID: 'leader' },
			{ userID: 'senior', name: 'Senior', hireDate: '2026-02-01', supervisorID: 'manager' }
		];

		expect(orderOrgchartPeopleByHierarchy(people).map((person) => person.userID)).toEqual([
			'leader',
			'peer',
			'manager',
			'senior',
			'junior'
		]);
		expect(people.map((person) => person.userID)).toEqual(['junior', 'manager', 'leader', 'peer', 'senior']);
	});

	test('treats employees with out-of-group supervisors as roots', () => {
		const people = [
			{ userID: 'internal-root', name: 'Internal Root', hireDate: '2026-02-01' },
			{ userID: 'external-root', name: 'External Root', hireDate: '2026-01-01', supervisorID: 'outside-group' }
		];

		expect(orderOrgchartPeopleByHierarchy(people).map((person) => person.userID)).toEqual(['external-root', 'internal-root']);
	});

	test('keeps cyclic records exactly once', () => {
		const people = [
			{ userID: 'cycle-b', name: 'Cycle B', hireDate: '2026-04-01', supervisorID: 'cycle-a' },
			{ userID: 'root', name: 'Root', hireDate: '2026-02-01' },
			{ userID: 'cycle-a', name: 'Cycle A', hireDate: '2026-03-01', supervisorID: 'cycle-b' }
		];

		expect(orderOrgchartPeopleByHierarchy(people).map((person) => person.userID)).toEqual(['root', 'cycle-a', 'cycle-b']);
	});
});
