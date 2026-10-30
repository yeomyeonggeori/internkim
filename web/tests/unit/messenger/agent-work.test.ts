import { describe, expect, test } from 'bun:test';
import {
	liveWorkAfterEvent,
	liveWorkLifetimeMilliseconds,
	liveWorkShownIn,
	liveWorkStillHeard,
	saidEventKind,
	workingEventKind,
	type LiveWork
} from '$lib/messenger/agent-work';

const heardAt = Date.parse('2026-10-30T01:00:00Z');
const conversationID = 'conversation-1';
const nothingLoaded = new Set<string>();

function working(work: Record<string, unknown>) {
	return { kind: workingEventKind, conversationID, work: { threadID: conversationID, ...work } };
}

function said(work: Record<string, unknown>) {
	return { kind: saidEventKind, conversationID, work: { threadID: conversationID, ...work } };
}

function heard(events: ReturnType<typeof working>[]): LiveWork[] {
	return events.reduce<LiveWork[]>((works, event) => liveWorkAfterEvent(works, event, conversationID, heardAt), []);
}

describe('liveWorkAfterEvent', () => {
	test('each piece of work gets its own bubble with the steps it reported', () => {
		const works = heard([
			working({ deliveryID: 'turn-A', steps: [{ title: 'event_list', status: 'in_progress' }] }),
			working({ deliveryID: 'turn-B', steps: [{ title: 'mail_message_search', status: 'pending' }] })
		]);

		expect(liveWorkShownIn(works, conversationID, nothingLoaded)).toEqual([
			{ key: 'turn-A', steps: [{ title: 'event_list', status: 'in_progress' }] },
			{ key: 'turn-B', steps: [{ title: 'mail_message_search', status: 'pending' }] }
		]);
	});

	test('the first work reported takes over the bubble shown since the question was asked', () => {
		const works = heard([working({ promptID: 'message-1' }), working({ deliveryID: 'turn-A', steps: [] })]);

		expect(works.map((work) => work.key)).toEqual(['turn-A']);
	});

	test('steps with a status nobody defined are left out', () => {
		const works = heard([working({ deliveryID: 'turn-A', steps: [{ title: 'event_list', status: 'thinking' }] })]);

		expect(works[0].steps).toEqual([]);
	});

	test('events of another conversation leave the bubbles as they were', () => {
		const event = { ...working({ deliveryID: 'turn-A' }), conversationID: 'conversation-2' };

		expect(liveWorkAfterEvent([], event, conversationID, heardAt)).toEqual([]);
	});
});

describe('liveWorkShownIn', () => {
	test('a bubble becomes the message it said once that message is loaded, and work that goes on starts a fresh bubble', () => {
		const works = heard([
			working({ deliveryID: 'turn-A', steps: [{ title: 'event_list', status: 'completed' }] }),
			said({ deliveryID: 'turn-A', messageID: 'checkpoint-A', isStillWorking: true })
		]);

		expect(liveWorkShownIn(works, conversationID, nothingLoaded)).toEqual([
			{ key: 'turn-A', steps: [{ title: 'event_list', status: 'completed' }] }
		]);
		expect(liveWorkShownIn(works, conversationID, new Set(['checkpoint-A']))).toEqual([{ key: 'turn-A', steps: [] }]);
	});

	test('the final answer ends the bubble once it is loaded', () => {
		const works = heard([working({ deliveryID: 'turn-A' }), said({ deliveryID: 'turn-A', messageID: 'answer-A', isStillWorking: false })]);

		expect(liveWorkShownIn(works, conversationID, new Set(['answer-A']))).toEqual([]);
	});

	test('work that stops without saying anything ends its bubble at once', () => {
		const works = heard([working({ deliveryID: 'turn-A' }), said({ deliveryID: 'turn-A', isStillWorking: false })]);

		expect(works).toEqual([]);
	});

	test('a bubble shows only where its answer will land', () => {
		const works = heard([working({ deliveryID: 'turn-A', threadID: 'message-7' })]);

		expect(liveWorkShownIn(works, 'message-7', nothingLoaded)).toHaveLength(1);
		expect(liveWorkShownIn(works, conversationID, nothingLoaded)).toEqual([]);
		expect(liveWorkShownIn(works, undefined, nothingLoaded)).toEqual([]);
	});
});

test('work that is never heard from again is forgotten after its lifetime', () => {
	const works = heard([working({ deliveryID: 'turn-A' })]);

	expect(liveWorkStillHeard(works, heardAt + liveWorkLifetimeMilliseconds - 1)).toBe(works);
	expect(liveWorkStillHeard(works, heardAt + liveWorkLifetimeMilliseconds)).toEqual([]);
});
