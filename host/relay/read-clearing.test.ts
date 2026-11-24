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
});

describe('telling the project a conversation was read', () => {
	test('asks it to clear that conversation on the member\'s devices', async () => {
		const asked: ReadClearing[] = [];
		const tell = readClearingTeller({
			askTheProject: async (request) => {
				asked.push(request);
				return { reached: 2 };
			},
			report: () => {}
		});

		expect(await tell('member-1', { conversationID: 'channel-1' })).toBe(2);
		expect(asked).toEqual([{ memberID: 'member-1', tag: 'message:channel-1' }]);
	});

	test('reports a project that would not answer and reaches nobody', async () => {
		const reported: string[] = [];
		const tell = readClearingTeller({
			askTheProject: async () => {
				throw new Error('the project answered 503 for clear-read-notifications');
			},
			report: (line) => reported.push(line)
		});

		expect(await tell('member-1', { conversationID: 'channel-1' })).toBe(0);
		expect(reported).toEqual(['notifications of a read conversation not cleared: the project answered 503 for clear-read-notifications']);
	});
});
