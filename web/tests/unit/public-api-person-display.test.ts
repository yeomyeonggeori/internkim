import { describe, expect, test } from 'bun:test';
import { answeredPerson } from '../../src/lib/server/public-api/record/people-tools';
import type { RecordPerson } from '../../src/lib/server/public-api/record/people';

function person(personID: string, name: string, supervisorID = ''): RecordPerson {
	return {
		personID,
		name,
		email: `${personID}@example.com`,
		isAdmin: false,
		employmentStatus: 'active',
		jobTitle: '',
		teamID: '',
		supervisorID,
		phoneNumber: '',
		hireDate: '',
		timeZone: ''
	};
}

describe('public API person display names', () => {
	test('keeps the stored name while localizing display and supervisor names', () => {
		const supervisor = person('supervisor', '샘플 이');
		const employee = person('employee', '예시 박', supervisor.personID);

		const answered = answeredPerson(employee, [employee, supervisor], [], 'ko');

		expect(answered.name).toBe('박예시');
		expect(employee.name).toBe('예시 박');
		expect(answered.mention).toBe('@박예시');
		expect(answered.supervisorName).toBe('이샘플');
	});
});
