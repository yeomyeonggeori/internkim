import { describe, expect, test } from 'bun:test';
import { readTyping, typingTeller } from './typing';

const heard = {
	conversationID: 'channel-1',
	authorExternalID: 'U-author',
	recipientExternalIDs: ['U-author', 'U-first', 'U-second']
};

function tellerThatKnows(members: Record<string, string>) {
	const delivered: { event: Record<string, unknown>; audience: string[] }[] = [];
	const lookups: string[][] = [];
	let now = 1_800_000_000_000;
	const tell = typingTeller({
		memberIDsOf: async (externalIDs) => {
			lookups.push(externalIDs);
			return new Map(externalIDs.filter((externalID) => members[externalID]).map((externalID) => [externalID, members[externalID]]));
		},
		deliver: (event, audience) => delivered.push({ event, audience }),
		now: () => now
	});
	return {
		tell,
		delivered,
		lookups,
		advance: (milliseconds: number) => {
			now += milliseconds;
		}
	};
}

describe('readTyping', () => {
	test('nobody is told that they themselves are typing', () => {
		expect(readTyping(heard)?.recipientExternalIDs).toEqual(['U-first', 'U-second']);
	});

	test('typing with no author or no conversation is not typing', () => {
		expect(readTyping({ ...heard, authorExternalID: '' })).toBeNull();
		expect(readTyping({ ...heard, conversationID: '' })).toBeNull();
		expect(readTyping(null)).toBeNull();
	});
});

describe('typingTeller', () => {
	test('tells only the members of the conversation, and says who is typing where', async () => {
		const { tell, delivered } = tellerThatKnows({ 'U-first': 'member-1', 'U-second': 'member-2' });

		expect(await tell(readTyping(heard)!)).toBe(2);
		expect(delivered).toEqual([
			{
				event: { kind: 'typing.started', conversationID: 'channel-1', authorExternalID: 'U-author' },
				audience: ['member-1', 'member-2']
			}
		]);
	});

	test('tells nobody when no recipient is a member, instead of telling everyone', async () => {
		const { tell, delivered } = tellerThatKnows({});

		expect(await tell(readTyping(heard)!)).toBe(0);
		expect(delivered).toEqual([]);
	});

	test('looks a person up once while they keep typing, and again after five minutes', async () => {
		const { tell, lookups, advance } = tellerThatKnows({ 'U-first': 'member-1' });
		const typing = readTyping({ ...heard, recipientExternalIDs: ['U-first'] })!;

		await tell(typing);
		await tell(typing);
		expect(lookups).toEqual([['U-first']]);

		advance(5 * 60_000);
		await tell(typing);
		expect(lookups).toEqual([['U-first'], ['U-first']]);
	});
});
