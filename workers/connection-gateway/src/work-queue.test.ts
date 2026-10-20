import { describe, expect, test } from 'bun:test';
import { WorkQueue, type QueuedWork } from './work-queue';

function work(requestID: string, affinityKey = 'person-1', deadline = 1_000): QueuedWork {
	return { requestID, affinityKey, work: { toolName: 'browser_open' }, deadline };
}

describe('the company work queue', () => {
	test('holds work until some box says it has room, then hands it to that box', () => {
		const queue = new WorkQueue();
		expect(queue.post(work('w1'))).toBeNull();
		expect(queue.waitingCount).toBe(1);
		const dispatched = queue.setFree('annex', 1);
		expect(dispatched.map((each) => [each.boxKey, each.item.requestID])).toEqual([['annex', 'w1']]);
		expect(queue.waitingCount).toBe(0);
	});

	test('never hands a box more work than the room it reported', () => {
		const queue = new WorkQueue();
		queue.setFree('annex', 1);
		expect(queue.post(work('w1'))?.boxKey).toBe('annex');
		expect(queue.post(work('w2', 'person-2'))).toBeNull();
	});

	test('takes the room a box reports as it stands, so a repeated report does not add room', () => {
		const queue = new WorkQueue();
		queue.setFree('annex', 1);
		queue.setFree('annex', 1);
		expect(queue.post(work('w1'))?.boxKey).toBe('annex');
		expect(queue.post(work('w2', 'person-2'))).toBeNull();
	});

	test('sends a person back to the box that last did their work while it has room', () => {
		const queue = new WorkQueue();
		queue.setFree('annex', 1);
		queue.post(work('w1', 'person-1'));
		queue.setFree('main', 4);
		queue.setFree('annex', 1);
		expect(queue.post(work('w2', 'person-1'))?.boxKey).toBe('annex');
	});

	test('gives the person to any box with room when their last box is full', () => {
		const queue = new WorkQueue();
		queue.setFree('annex', 1);
		queue.post(work('w1', 'person-1'));
		queue.setFree('main', 1);
		expect(queue.post(work('w2', 'person-1'))?.boxKey).toBe('main');
	});

	test('stops offering a box that went away', () => {
		const queue = new WorkQueue();
		queue.setFree('annex', 3);
		queue.forget('annex');
		expect(queue.post(work('w1'))).toBeNull();
	});

	test('lets work that waited past its deadline go, and only that work', () => {
		const queue = new WorkQueue();
		queue.post(work('late', 'person-1', 100));
		queue.post(work('fresh', 'person-2', 900));
		expect(queue.expire(500).map((item) => item.requestID)).toEqual(['late']);
		expect(queue.waitingCount).toBe(1);
	});
});
