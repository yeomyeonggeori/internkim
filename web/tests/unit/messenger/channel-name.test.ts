import { describe, expect, test } from 'bun:test';
import { channelName } from '$lib/messenger/channel-name';
import { personKey, type MessengerDirectory } from '$lib/messenger/messenger-directory';
import type { MessengerChannel, MessengerPerson } from '$lib/messenger/messenger-api';

const directory: MessengerDirectory = {
	nameOfMember: new Map([['member-1', '예시 김']]),
	nameOfExternal: new Map([
		['external-1', '예시 김'],
		['bot-1', 'Intern Kim']
	]),
	memberOfExternal: new Map([['external-1', 'member-1']]),
	externalsOfMember: new Map([['member-1', ['external-1']]]),
	memberOfEmail: new Map()
};

// The reader holds an account on each messenger the company uses, and a
// conversation names them by whichever one it came from.
const readersAccounts = new Set(['member:reader', 'external:mattermost-reader', 'external:buzz-reader']);

function readerIs(person: MessengerPerson): boolean {
	return readersAccounts.has(personKey(person));
}

function conversationWith(externalIDs: string[], name: string): MessengerChannel {
	return {
		id: 'channel-1',
		platform: 'mattermost',
		name,
		isDirect: true,
		isPrivate: true,
		position: 0,
		participants: externalIDs.map((externalID) => ({ externalID }))
	};
}

describe('channelName', () => {
	test('calls a direct conversation what the company calls that person', () => {
		const conversation = conversationWith(['external-1', 'mattermost-reader'], '예시 김');
		expect(channelName(conversation, directory, readerIs, 'ko', '김인턴')).toBe('김예시');
	});

	test('falls back to the messenger for somebody the record does not know', () => {
		const conversation = conversationWith(['bot-1', 'mattermost-reader'], 'Intern Kim');
		expect(channelName(conversation, directory, readerIs, 'ko', '김인턴')).toBe('Intern Kim');
	});

	test('calls the agent by the name the product goes by in this language', () => {
		const conversation = { ...conversationWith(['bot-1', 'mattermost-reader'], 'Intern Kim'), isWithTheAgent: true };
		expect(channelName(conversation, directory, readerIs, 'ko', '김인턴')).toBe('김인턴');
		expect(channelName(conversation, directory, readerIs, 'en', 'internkim')).toBe('internkim');
	});

	test('leaves a group conversation with the name it was given', () => {
		const conversation = { ...conversationWith([], '광장'), isDirect: false };
		expect(channelName(conversation, directory, readerIs, 'ko', '김인턴')).toBe('광장');
	});

	test('takes the reader out however the conversation named them', () => {
		const conversation = conversationWith(['external-1', 'buzz-reader'], 'a name nobody chose');
		expect(channelName(conversation, directory, readerIs, 'ko', '김인턴')).toBe('김예시');
	});

	test('never leaves a conversation called after the person reading it', () => {
		const conversation = conversationWith(['buzz-reader'], 'buzz-reader');
		expect(channelName(conversation, directory, readerIs, 'ko', '김인턴')).not.toBe('김예시');
		expect(channelName(conversation, directory, readerIs, 'ko', '김인턴')).toBe('buzz-reader');
	});
});
