import { describe, expect, test } from 'bun:test';
import type { ChannelSummary } from '../../../src/lib/components/channel/channel-api';
import type { CompanyEvent } from '../../../src/lib/company-event';
import {
	hiddenConversationAnsweredBy,
	shownConversations
} from '../../../src/lib/messenger/hidden-direct-messages';

const hidden: ChannelSummary = {
	id: 'dm-hidden',
	name: '박예시',
	kind: 'dm',
	isHidden: true,
	counterpart: { externalID: 'counterpart' }
};
const shown: ChannelSummary = { id: 'dm-shown', name: '최견본', kind: 'dm', counterpart: { externalID: 'other' } };
const conversations = [hidden, shown];

function arrival(conversationID: string, authorExternalID?: string): CompanyEvent {
	return { kind: 'message.arrived', conversationID, messageID: 'message-1', authorExternalID };
}

describe('which direct conversations the list shows', () => {
	test('leaves out the ones the person hid', () => {
		expect(shownConversations(conversations).map((conversation) => conversation.id)).toEqual(['dm-shown']);
	});
});

describe('which arrival brings a hidden conversation back', () => {
	test('the other person writing in it', () => {
		expect(hiddenConversationAnsweredBy(conversations, arrival('dm-hidden', 'counterpart'))?.id).toBe('dm-hidden');
	});

	test('not the person writing in it from another device', () => {
		expect(hiddenConversationAnsweredBy(conversations, arrival('dm-hidden', 'me'))).toBeUndefined();
	});

	test('not an arrival that does not say who wrote it', () => {
		expect(hiddenConversationAnsweredBy(conversations, arrival('dm-hidden'))).toBeUndefined();
	});

	test('not a conversation that is already shown', () => {
		expect(hiddenConversationAnsweredBy(conversations, arrival('dm-shown', 'other'))).toBeUndefined();
	});
});
