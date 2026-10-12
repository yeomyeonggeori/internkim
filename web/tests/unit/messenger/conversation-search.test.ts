import { afterAll, describe, expect, mock, test } from 'bun:test';
import { searchExcerpt } from '../../../src/lib/messenger/conversation-search';

const asked: { capability: string; body: Record<string, unknown> }[] = [];

const hostBridge = { ...(await import('../../../src/lib/host-bridge')) };

afterAll(() => {
	mock.module('../../../src/lib/host-bridge', () => hostBridge);
});

mock.module('../../../src/lib/host-bridge', () => ({
	...hostBridge,
	onCompanyEvent: () => () => undefined,
	callCompanyApp: async ({ capability, body }: { capability: string; body: Record<string, unknown> }) => {
		asked.push({ capability, body });
		return {
			status: 200,
			body: {
				matches: [
					{
						id: 'message-1',
						conversationID: 'channel-1',
						parentID: 'root-1',
						authorExternalID: 'person-a',
						body: '회의록 올렸어요',
						postedAt: '2026-10-12T05:00:00.000Z'
					}
				]
			}
		};
	}
}));

const { searchPosts } = await import('../../../src/lib/messenger/messenger-api');

describe('searching the open conversation', () => {
	test('asks person.messages.search for that conversation and what was typed', async () => {
		asked.length = 0;

		const matches = await searchPosts('channel-1', '회의록');

		expect(asked).toEqual([{ capability: 'person.messages.search', body: { conversationID: 'channel-1', query: '회의록' } }]);
		expect(matches).toEqual([
			{
				id: 'message-1',
				channelID: 'channel-1',
				parentID: 'root-1',
				author: { externalID: 'person-a' },
				body: '회의록 올렸어요',
				postedAt: '2026-10-12T05:00:00.000Z'
			}
		]);
	});
});

describe('the excerpt a search result shows', () => {
	test('marks the words that matched, whatever case they were typed in', () => {
		expect(searchExcerpt('The Weekly Report is up', 'weekly report')).toEqual({
			before: 'The ',
			match: 'Weekly Report',
			after: ' is up'
		});
	});

	test('starts near the match when the message is long, and says it was cut', () => {
		const excerpt = searchExcerpt(`${'가'.repeat(60)} 회의록 공유합니다`, '회의록');

		expect(excerpt.before.startsWith('…')).toBe(true);
		expect(excerpt.before.length).toBeLessThanOrEqual(25);
		expect(excerpt.match).toBe('회의록');
		expect(excerpt.after).toBe(' 공유합니다');
	});

	test('folds line breaks so a match across them is still marked', () => {
		expect(searchExcerpt('어제\n회의록', '어제 회의록').match).toBe('어제 회의록');
	});
});
