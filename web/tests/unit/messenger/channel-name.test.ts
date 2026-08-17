import { describe, expect, test } from 'bun:test';
import { channelName } from '$lib/messenger/channel-name';
import { personKey, type MessengerDirectory } from '$lib/messenger/messenger-directory';
import type { MessengerChannel, MessengerPerson } from '$lib/messenger/messenger-api';

function canonicalKeyForTest(person: MessengerPerson, people: MessengerDirectory): string {
	const memberID = person.memberID ?? (person.externalID ? people.memberOfExternal.get(person.externalID) : undefined);
	return memberID ? personKey({ memberID }) : personKey(person);
}

const directory: MessengerDirectory = {
	nameOfMember: new Map([['member-1', '김여명']]),
	nameOfExternal: new Map([
		['external-1', '여명 김'],
		['bot-1', 'Intern Kim']
	]),
	memberOfExternal: new Map([['external-1', 'member-1']]),
	externalOfMember: new Map([['member-1', 'external-1']]),
	memberOfEmail: new Map()
};

function conversationWith(externalIDs: string[], name: string): MessengerChannel {
	return {
		id: 'channel-1',
		platform: 'mattermost',
		name,
		isDirect: true,
		position: 0,
		participants: externalIDs.map((externalID) => ({ externalID }))
	};
}

describe('channelName', () => {
	test('calls a direct conversation what the company calls that person', () => {
		const conversation = conversationWith(['external-1', 'mine'], '여명 김');
		expect(channelName(conversation, directory, 'external:mine', canonicalKeyForTest)).toBe('김여명');
	});

	test('falls back to the messenger for somebody the record does not know', () => {
		const conversation = conversationWith(['bot-1', 'mine'], 'Intern Kim');
		expect(channelName(conversation, directory, 'external:mine', canonicalKeyForTest)).toBe('Intern Kim');
	});

	test('leaves a group conversation with the name it was given', () => {
		const conversation = { ...conversationWith([], '광장'), isDirect: false };
		expect(channelName(conversation, directory, 'external:mine', canonicalKeyForTest)).toBe('광장');
	});
});
