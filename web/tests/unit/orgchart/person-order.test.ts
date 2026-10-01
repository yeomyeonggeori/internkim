import { describe, expect, test } from 'bun:test';
import { compareOrganizationPeople, orderOrganizationPeopleByHierarchy } from '../../../src/lib/organization/person-order';

describe('organization person ordering', () => {
	test('orders by hire date before name and puts missing dates after dated people', () => {
		const people = [
			{ memberID: 'missing-zara', name: 'Zara' },
			{ memberID: 'newer-aaron', name: 'Aaron', hireDate: '2026-04-01' },
			{ memberID: 'older-min', name: 'Min', hireDate: '2026-02-01' },
			{ memberID: 'missing-ada', name: 'Ada' },
			{ memberID: 'newer-bora', name: 'Bora', hireDate: '2026-04-01' }
		];

		expect([...people].sort(compareOrganizationPeople).map((person) => person.memberID)).toEqual([
			'older-min',
			'newer-aaron',
			'newer-bora',
			'missing-ada',
			'missing-zara'
		]);
	});

	test('orders Korean names before English names on the same hire date', () => {
		const people = [
			{ memberID: 'pptx', name: 'PPTX Tester', hireDate: '2026-04-01' },
			{ memberID: 'sample', name: '이샘플', hireDate: '2026-04-01' },
			{ memberID: 'yeomyeong', name: '김예시', hireDate: '2026-04-01' },
			{ memberID: 'aaron', name: 'Aaron', hireDate: '2026-04-01' }
		];

		expect([...people].sort(compareOrganizationPeople).map((person) => person.memberID)).toEqual([
			'yeomyeong',
			'sample',
			'aaron',
			'pptx'
		]);
	});

	test('orders Korean names before English names when hire dates are missing', () => {
		const people = [
			{ memberID: 'pptx', name: 'PPTX Tester' },
			{ memberID: 'sample', name: '이샘플' },
			{ memberID: 'yeomyeong', name: '김예시' },
			{ memberID: 'aaron', name: 'Aaron' }
		];

		expect([...people].sort(compareOrganizationPeople).map((person) => person.memberID)).toEqual([
			'yeomyeong',
			'sample',
			'aaron',
			'pptx'
		]);
	});

	test('places each supervisor before recursively ordered direct reports', () => {
		const people = [
			{ memberID: 'junior', name: 'Junior', hireDate: '2026-03-01', supervisorID: 'manager' },
			{ memberID: 'manager', name: 'Manager', hireDate: '2026-04-01', supervisorID: 'leader' },
			{ memberID: 'leader', name: 'Leader', hireDate: '2026-05-01' },
			{ memberID: 'peer', name: 'Peer', hireDate: '2026-01-01', supervisorID: 'leader' },
			{ memberID: 'senior', name: 'Senior', hireDate: '2026-02-01', supervisorID: 'manager' }
		];

		expect(orderOrganizationPeopleByHierarchy(people).map((person) => person.memberID)).toEqual([
			'leader',
			'peer',
			'manager',
			'senior',
			'junior'
		]);
		expect(people.map((person) => person.memberID)).toEqual(['junior', 'manager', 'leader', 'peer', 'senior']);
	});

	test('treats employees with out-of-group supervisors as roots', () => {
		const people = [
			{ memberID: 'internal-root', name: 'Internal Root', hireDate: '2026-02-01' },
			{ memberID: 'external-root', name: 'External Root', hireDate: '2026-01-01', supervisorID: 'outside-group' }
		];

		expect(orderOrganizationPeopleByHierarchy(people).map((person) => person.memberID)).toEqual(['external-root', 'internal-root']);
	});

	test('keeps cyclic records exactly once', () => {
		const people = [
			{ memberID: 'cycle-b', name: 'Cycle B', hireDate: '2026-04-01', supervisorID: 'cycle-a' },
			{ memberID: 'root', name: 'Root', hireDate: '2026-02-01' },
			{ memberID: 'cycle-a', name: 'Cycle A', hireDate: '2026-03-01', supervisorID: 'cycle-b' }
		];

		expect(orderOrganizationPeopleByHierarchy(people).map((person) => person.memberID)).toEqual(['root', 'cycle-a', 'cycle-b']);
	});
});
