import { describe, expect, test } from 'bun:test';
import { createTaskLoadTracker } from '../../src/routes/task/task-load-tracker';

describe('createTaskLoadTracker', () => {
	test('keeps only the latest flow load active', () => {
		const tracker = createTaskLoadTracker();

		const firstLoad = tracker.start();
		const secondLoad = tracker.start();

		expect(tracker.isCurrent(firstLoad)).toBe(false);
		expect(tracker.isCurrent(secondLoad)).toBe(true);
	});
});
