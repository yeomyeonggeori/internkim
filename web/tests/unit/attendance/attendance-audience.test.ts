import { describe, expect, test } from 'bun:test';
import { whoHearsTheClockOf, type Colleague } from '../../../../supabase/functions/_shared/attendance-audience.ts';
import type { AttendanceReach } from '../../../../supabase/functions/_shared/categories.ts';

function colleague(id: string, teamID: string | null, supervisorID: string | null, attendanceReach: AttendanceReach = 'everyone'): Colleague {
	return { id, teamID, supervisorID, attendanceReach };
}

const chief = colleague('chief', 'development', 'ceo');
const ceo = colleague('ceo', 'executive', null);
const thirdLead = colleague('third-lead', 'development-3', 'chief');
const firstLead = colleague('first-lead', 'development-1', 'chief');
const advisor = colleague('advisor', 'research', null);

function listenerWith(attendanceReach: AttendanceReach): Colleague {
	return colleague('listener', 'development-3', 'third-lead', attendanceReach);
}

function hears(listener: Colleague, announcer: Colleague, others: Colleague[] = []): boolean {
	const company = [listener, announcer, ...others.filter((other) => other.id !== announcer.id)];
	return whoHearsTheClockOf(announcer.id, company).includes(listener.id);
}

describe('whose clock a person is told about', () => {
	test('everyone hears the whole company', () => {
		expect(hears(listenerWith('everyone'), advisor)).toBe(true);
	});

	test('team hears their own team and nobody above it', () => {
		const teammate = colleague('teammate', 'development-3', 'third-lead');

		expect(hears(listenerWith('team'), teammate)).toBe(true);
		expect(hears(listenerWith('team'), chief, [thirdLead])).toBe(false);
	});

	test('team and supervisors hears every supervisor up to the top', () => {
		const company = [thirdLead, chief, ceo];

		expect(hears(listenerWith('teamAndSupervisors'), thirdLead, company)).toBe(true);
		expect(hears(listenerWith('teamAndSupervisors'), chief, company)).toBe(true);
		expect(hears(listenerWith('teamAndSupervisors'), ceo, company)).toBe(true);
	});

	test('team and supervisors does not hear a lead who supervises somebody else', () => {
		expect(hears(listenerWith('teamAndSupervisors'), firstLead, [thirdLead, chief])).toBe(false);
	});

	test('a supervisor in another team is heard all the same', () => {
		const listener = colleague('listener', 'development-3', 'advisor', 'teamAndSupervisors');

		expect(hears(listener, advisor)).toBe(true);
	});

	test('nobody without a team shares one with anybody', () => {
		const loner = colleague('loner', null, null, 'team');

		expect(hears(loner, colleague('other-loner', null, null))).toBe(false);
	});

	test('supervisors who supervise each other end the climb', () => {
		const first = colleague('first', 'a', 'second');
		const second = colleague('second', 'b', 'first');
		const listener = colleague('listener', 'c', 'first', 'teamAndSupervisors');

		expect(whoHearsTheClockOf('second', [listener, first, second])).toEqual(['listener', 'first']);
	});

	test('the person who clocked is never told about it', () => {
		expect(whoHearsTheClockOf('chief', [chief, ceo])).toEqual(['ceo']);
	});
});
