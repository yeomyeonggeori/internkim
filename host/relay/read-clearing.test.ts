import { describe, expect, test } from 'bun:test';
import { readClearingOf, readClearingTeller, type ReadClearing } from './read-clearing';

describe('a read conversation', () => {
	test('is cleared by the tag its message notifications carry', () => {
		expect(readClearingOf('member-1', { conversationID: ' channel-1 ', messageID: 'event-1' })).toEqual({
			memberID: 'member-1',
			tag: 'message:channel-1'
		});
	});

	test('names no conversation, so nothing is cleared', () => {
		expect(readClearingOf('member-1', { messageID: 'event-1' })).toBeNull();
		expect(readClearingOf('', { conversationID: 'channel-1' })).toBeNull();
	});

	test('read only in a thread leaves the conversation\'s notifications in place', () => {
		expect(readClearingOf('member-1', { conversationID: 'channel-1', messageID: 'reply-1', parentID: 'root-1' })).toBeNull();
		expect(readClearingOf('member-1', { conversationID: 'channel-1', messageID: 'event-1', parentID: '' })).toEqual({
			memberID: 'member-1',
			tag: 'message:channel-1'
		});
	});
});

type Scheduled = { run: () => void; milliseconds: number; cancelled: boolean };

function tellerWithClock(askTheProject: (request: ReadClearing) => Promise<{ reached?: number }>, report: (line: string) => void = () => {}) {
	const scheduled: Scheduled[] = [];
	const teller = readClearingTeller({
		askTheProject,
		report,
		quietMilliseconds: 30_000,
		schedule: (run, milliseconds) => {
			const entry: Scheduled = { run, milliseconds, cancelled: false };
			scheduled.push(entry);
			return () => {
				entry.cancelled = true;
			};
		}
	});
	const elapse = async (): Promise<void> => {
		for (const entry of scheduled.splice(0)) if (!entry.cancelled) entry.run();
		await Promise.resolve();
	};
	return { tell: teller.read, arrived: teller.arrived, scheduled, elapse };
}

describe('telling the project a conversation was read', () => {
	test('asks it to clear that conversation once the reading has been quiet', async () => {
		const asked: ReadClearing[] = [];
		const { tell, scheduled, elapse } = tellerWithClock(async (request) => {
			asked.push(request);
			return { reached: 2 };
		});

		tell('member-1', { conversationID: 'channel-1' });
		expect(asked).toEqual([]);
		expect(scheduled.map((entry) => entry.milliseconds)).toEqual([30_000]);

		await elapse();
		expect(asked).toEqual([{ memberID: 'member-1', tag: 'message:channel-1' }]);
	});

	test('a conversation read message after message is cleared once', async () => {
		const asked: ReadClearing[] = [];
		const { tell, elapse } = tellerWithClock(async (request) => {
			asked.push(request);
			return {};
		});

		tell('member-1', { conversationID: 'channel-1', messageID: 'event-1' });
		tell('member-1', { conversationID: 'channel-1', messageID: 'event-2' });
		tell('member-1', { conversationID: 'channel-1', messageID: 'event-3' });
		tell('member-1', { conversationID: 'channel-2', messageID: 'event-4' });

		await elapse();
		expect(asked).toEqual([
			{ memberID: 'member-1', tag: 'message:channel-1' },
			{ memberID: 'member-1', tag: 'message:channel-2' }
		]);
	});

	test('a message arriving after the reading keeps its notification', async () => {
		const asked: ReadClearing[] = [];
		const { tell, arrived, elapse } = tellerWithClock(async (request) => {
			asked.push(request);
			return {};
		});

		tell('member-1', { conversationID: 'channel-1', messageID: 'event-1' });
		tell('member-1', { conversationID: 'channel-2', messageID: 'event-2' });
		arrived('channel-1');

		await elapse();
		expect(asked).toEqual([{ memberID: 'member-1', tag: 'message:channel-2' }]);
	});

	test('reports a project that would not answer', async () => {
		const reported: string[] = [];
		const { tell, elapse } = tellerWithClock(
			async () => {
				throw new Error('the project answered 503 for clear-read-notifications');
			},
			(line) => reported.push(line)
		);

		tell('member-1', { conversationID: 'channel-1' });
		await elapse();
		await Promise.resolve();
		expect(reported).toEqual(['notifications of a read conversation not cleared: the project answered 503 for clear-read-notifications']);
	});
});
