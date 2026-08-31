import { describe, expect, test } from 'bun:test';
import { centralTaskStatusOptions } from '../../../src/lib/task/central-task';
import { centralTaskStatuses } from '../../../../supabase/functions/_shared/central-task-status.ts';

describe('the shared task status copy stays interchangeable with the web one', () => {
	test('both name the same statuses', () => {
		expect([...centralTaskStatuses].sort()).toEqual([...centralTaskStatusOptions].sort());
	});
});
