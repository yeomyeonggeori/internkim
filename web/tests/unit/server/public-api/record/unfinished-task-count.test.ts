import { describe, expect, test } from 'bun:test';
import { countUnfinishedTasks } from '../../../../../src/lib/server/public-api/record/unfinished-task-count';

function task(status: string) {
	return { status };
}

describe('countUnfinishedTasks', () => {
	test('counts unfinished statuses and excludes completed statuses', () => {
		expect(
			countUnfinishedTasks([
				task('completed'),
				task('rejected'),
				task('stopped'),
				task('requested'),
				task('planned'),
				task('in_progress'),
				task('paused')
			])
		).toBe(4);
	});

});
