import { describe, expect, test } from 'bun:test';
import type { TaskReach } from '../../../../supabase/functions/_shared/categories.ts';
import { whoHearsTheCompletionOf, type TaskListener } from '../../../../supabase/functions/_shared/task-audience.ts';

function colleague(id: string, teamID: string | null, supervisorID: string | null, taskReach: TaskReach = 'team'): TaskListener {
	return { id, teamID, supervisorID, taskReach };
}

const lead = colleague('lead', 'development', 'chief');
const chief = colleague('chief', 'executive', null);
const teammate = colleague('teammate', 'development', 'lead');
const outsider = colleague('outsider', 'sales', 'chief');

function listenerWith(taskReach: TaskReach): TaskListener {
	return colleague('listener', 'development', 'lead', taskReach);
}

function hears(listener: TaskListener, finisher: TaskListener, others: TaskListener[] = []): boolean {
	const company = [listener, finisher, ...others.filter((other) => other.id !== finisher.id)];
	return whoHearsTheCompletionOf(finisher.id, company).includes(listener.id);
}

describe('whose finished work a person is told about', () => {
	test('team hears a teammate and nobody outside the team', () => {
		expect(hears(listenerWith('team'), teammate)).toBe(true);
		expect(hears(listenerWith('team'), outsider)).toBe(false);
		expect(hears(listenerWith('team'), chief, [lead])).toBe(false);
	});

	test('teamAndSupervisors also hears every supervisor above', () => {
		expect(hears(listenerWith('teamAndSupervisors'), chief, [lead])).toBe(true);
		expect(hears(listenerWith('teamAndSupervisors'), outsider, [lead, chief])).toBe(false);
	});

	test('everyone hears the whole company', () => {
		expect(hears(listenerWith('everyone'), outsider)).toBe(true);
	});

	test('involved hears nobody, a teammate included', () => {
		expect(hears(listenerWith('involved'), teammate)).toBe(false);
	});

	test('nobody hears their own finished work', () => {
		expect(whoHearsTheCompletionOf('teammate', [teammate, colleague('other', 'development', 'lead')])).toEqual(['other']);
	});
});
