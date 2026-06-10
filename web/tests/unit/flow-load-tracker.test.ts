import { describe, expect, test } from 'bun:test';
import { createFlowLoadTracker } from '../../src/routes/flow/flow-load-tracker';

describe('createFlowLoadTracker', () => {
	test('keeps only the latest flow load active', () => {
		const tracker = createFlowLoadTracker();

		const firstLoad = tracker.start();
		const secondLoad = tracker.start();

		expect(tracker.isCurrent(firstLoad)).toBe(false);
		expect(tracker.isCurrent(secondLoad)).toBe(true);
	});
});
