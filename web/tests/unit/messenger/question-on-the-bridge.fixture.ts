import { mock } from 'bun:test';

const readerExternalID = 'reader-pubkey';
const colleagueExternalID = 'colleague-pubkey';
const agentExternalID = 'agent-pubkey';
const sent: Record<string, unknown>[] = [];

const options = [
	{ key: 'morning', label: '오전 10시', isRecommended: true },
	{ key: 'afternoon', label: '오후 3시', isRecommended: false }
];

const approvalOptions = [
	{ key: 'approve', label: '', isRecommended: false },
	{ key: 'reject', label: '', isRecommended: false }
];

function message(id: string, fields: Record<string, unknown>) {
	return {
		id,
		conversationID: 'dm-1',
		authorExternalID: agentExternalID,
		body: '언제가 좋을까요?\n\n1. 오전 10시\n2. 오후 3시',
		postedAt: '2026-11-11T01:00:00.000Z',
		reactions: [],
		attachments: [],
		...fields
	};
}

const messages = [
	message('asks-the-reader', {
		ask: { interactionID: 'interaction-1', kind: 'choice', targetExternalID: readerExternalID, options }
	}),
	message('asks-a-colleague', {
		ask: { interactionID: 'interaction-2', kind: 'approval', targetExternalID: colleagueExternalID, options: approvalOptions }
	}),
	message('asks-anyone', {
		ask: { interactionID: 'interaction-3', kind: 'choice', targetExternalID: '', options, resolution: { key: 'morning' } }
	}),
	message('answered-in-own-words', {
		ask: { interactionID: 'interaction-4', kind: 'choice', targetExternalID: readerExternalID, options, resolution: {} }
	}),
	message('asks-in-a-broken-shape', { ask: { interactionID: 'interaction-5', kind: 'poll', targetExternalID: '', options } }),
	message('failed', { body: '일정을 만들지 못했어요. 캘린더 연결을 확인해 주세요.', isFailure: true }),
	message('plain', { body: '안녕하세요' })
];

mock.module('$lib/host-bridge', () => ({
	callCompanyApp: async ({ capability, body }: { capability: string; body: Record<string, unknown> }) => {
		if (capability === 'person.messages.list') return { status: 200, body: { messages, hasMoreBefore: false } };
		sent.push(body);
		return { status: 200, body: message('reply', { authorExternalID: readerExternalID, body: String(body.body) }) };
	},
	onCompanyEvent: () => () => undefined
}));
const supabaseModule = { ...(await import('$lib/supabase')) };
mock.module('$lib/supabase', () => ({ ...supabaseModule, isSupabaseConfigured: () => true }));
mock.module('$lib/supabase-session', () => ({ supabaseMember: async () => ({ memberID: 'reader' }) }));
mock.module('$lib/person-name.svelte', () => ({ currentPersonNameLocale: () => 'ko' }));
mock.module('$lib/messenger/cache-scope', () => ({
	messengerCacheScope: async () => ({ key: 'scope', generation: 0 }),
	requireCurrentMessengerScope: async () => {},
	onMessengerCacheReset: () => () => {}
}));
mock.module('$lib/stores/custom-emoji.svelte', () => ({ customEmoji: { nameToURL: new Map() } }));
mock.module('$lib/stores/person-picture.svelte', () => ({ personPicture: {} }));
mock.module('$lib/stores/attachment-source.svelte', () => ({ attachmentSource: {} }));
mock.module('$lib/messenger/messenger-people', () => ({
	fetchMessengerPeople: async () => ({
		nameOfMember: new Map([['reader', '이샘플'], ['colleague', '박예시']]),
		nameOfExternal: new Map(),
		memberOfExternal: new Map([[readerExternalID, 'reader'], [colleagueExternalID, 'colleague']]),
		externalsOfMember: new Map([['reader', [readerExternalID]], ['colleague', [colleagueExternalID]]]),
		memberOfEmail: new Map(),
		accounts: { people: [], agentExternalID }
	}),
	agentName: () => '김인턴',
	isTheAgent: () => false,
	messengerPersonLabel: () => ''
}));

const originalWarn = console.warn;
const warnings: string[] = [];
console.warn = (description: string) => warnings.push(description);

const { bridgeConversationRecord, bridgeSendMessage } = await import('../../../src/lib/messenger/channel-over-bridge');
const conversation = await bridgeConversationRecord('dm-1');
await bridgeSendMessage('오전 10시', 'dm-1', undefined, [], undefined, { interactionID: 'interaction-1', key: 'morning' });

console.warn = originalWarn;
console.log(
	JSON.stringify({
		currentUserID: conversation.currentUserID,
		messages: Object.fromEntries(
			conversation.messages.map((each) => [each.id, { interaction: each.interaction ?? null, isError: each.isError ?? null }])
		),
		warnings,
		sentAnswer: sent[0]?.answer ?? null,
		sentBody: sent[0]?.body ?? null
	})
);
