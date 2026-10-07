import { mock } from 'bun:test';

const scenario = process.argv[2];
const nextTurn = () => new Promise<void>((resolve) => setTimeout(resolve, 0));

if (scenario === 'mail') {
	Reflect.set(globalThis, '$state', <Value>(value: Value): Value => value);
	const toolDelayMilliseconds = Number(process.argv[3] ?? 0);
	let accounts = 0;
	let mailboxes = 0;
	let messages = 0;
	mock.module('$lib/supabase', () => ({ isSupabaseConfigured: () => true }));
	mock.module('../../src/routes/mail/mail-account-api', () => ({
		recordMailAccount: async () => {
			accounts += 1;
			return { email: 'member@example.com', isConfigured: true, defaultMailbox: 'INBOX' };
		},
		keepRecordMailAccount: async () => ({}),
		testRecordMailAccount: async () => ({})
	}));
	mock.module('$lib/public-api-call', () => ({
		invokeTool: async (name: string) => {
			if (toolDelayMilliseconds > 0) await new Promise((resolve) => setTimeout(resolve, toolDelayMilliseconds));
			if (name === 'mail_mailbox_list') {
				mailboxes += 1;
				return { mailboxes: [{ name: 'INBOX', displayName: 'Inbox', unseen: 0, total: 1 }] };
			}
			if (name === 'mail_message_list') {
				messages += 1;
				return { messages: [{ uid: 1, mailbox: 'INBOX', subject: 'Fresh message', isRead: true }], nextCursor: '' };
			}
			return {};
		}
	}));
	const { createMailPageController } = await import('../../src/routes/mail/mail-page-controller.svelte');
	const { mailText } = await import('../../src/routes/mail/text');
	const controller = createMailPageController(mailText.en);
	controller.canSelectFirstMessage = false;
	let startedAt = performance.now();
	const snapshot = () => ({ accounts, mailboxes, messages, visible: controller.messages[0]?.subject,
		...(toolDelayMilliseconds > 0 ? { toolDelayMilliseconds, listReadyMilliseconds: Math.round(performance.now() - startedAt) } : {}) });
	await controller.loadMail();
	const first = snapshot();
	startedAt = performance.now();
	await controller.loadMail();
	console.log(JSON.stringify({ first, refresh: snapshot() }));
} else if (scenario === 'directory') {
	mock.module('$lib/messenger/cache-scope', () => ({ messengerCacheScope: async () => ({ key: 'scope', generation: 0 }), requireCurrentMessengerScope: async () => {}, onMessengerCacheReset: () => () => {} }));
	const started: string[] = [];
	let releaseMembers = () => {};
	const members = new Promise<{ data: []; error: null }>((resolve) => {
		releaseMembers = () => resolve({ data: [], error: null });
	});
	mock.module('$lib/supabase', () => ({
		supabase: () => ({
			from: (table: string) => {
				const query = {
					select: () => query,
					neq: () => query,
					returns: () => {
						started.push(table);
						return table === 'member' ? members : Promise.resolve({ data: [], error: null });
					}
				};
				return query;
			}
		})
	}));
	const { fetchMessengerDirectory } = await import('../../src/lib/messenger/messenger-directory');
	const reading = fetchMessengerDirectory();
	await nextTurn();
	const beforeMembers = [...started];
	releaseMembers();
	const directory = await reading;
	console.log(JSON.stringify({ started: beforeMembers, members: directory.nameOfMember.size }));
} else if (scenario === 'file-list') {
	const { readFileSync } = await import('node:fs');
	const { compile, compileModule } = await import('svelte/compiler');
	const { Window } = await import('happy-dom');
	const browser = new Window();
	for (const name of Object.getOwnPropertyNames(browser)) {
		if (name in globalThis) continue;
		Object.defineProperty(globalThis, name, { value: Reflect.get(browser, name), configurable: true, writable: true });
	}
	Object.assign(globalThis, { window: browser, document: browser.document });
	Bun.plugin({ name: 'svelte', setup(build) {
		build.onLoad({ filter: /\.svelte$/ }, ({ path }) => ({ contents: compile(readFileSync(path, 'utf8'), { filename: path, generate: 'client' }).js.code, loader: 'js' }));
		build.onLoad({ filter: /\.svelte\.ts$/ }, ({ path }) => ({ contents: compileModule(new Bun.Transpiler({ loader: 'ts' }).transformSync(readFileSync(path, 'utf8')), { filename: path, generate: 'client' }).js.code, loader: 'js' }));
	} });
	const { flushSync, mount, unmount } = await import('svelte');
	const { default: FileBrowserList } = await import('../../src/lib/components/file-browser-list.svelte');
	const target = document.createElement('div');
	document.body.appendChild(target);
	const component = mount(FileBrowserList, { target, props: {
		entries: [{ id: 'folder', name: 'Available folder', isDirectory: true, secondary: '', date: '' }],
		isLoading: true, title: 'Files', nameLabel: 'Name', secondaryLabel: 'Type', dateLabel: 'Date', emptyLabel: 'Empty', onSelect: () => {}
	} });
	flushSync();
	console.log(JSON.stringify({ visible: target.querySelector('button')?.textContent?.trim(), busy: target.querySelector('section')?.getAttribute('aria-busy') }));
	await unmount(component);
} else if (scenario === 'emoji') {
	mock.module('$lib/messenger/cache-scope', () => ({ onMessengerCacheReset: () => () => {} }));
	Reflect.set(globalThis, '$state', <Value>(value: Value): Value => value);
	let requests = 0;
	let release = () => {};
	const names = new Promise<string[]>((resolve) => { release = () => resolve(['sample']); });
	mock.module('$lib/supabase', () => ({ isSupabaseConfigured: () => true }));
	mock.module('$lib/messenger/messenger-api', () => ({
		fetchCustomEmojiNames: () => { requests += 1; return names; },
		fetchCustomEmojiImage: async () => ({ dataURL: 'data:image/png;base64,test' })
	}));
	const { customEmoji } = await import('../../src/lib/stores/custom-emoji.svelte');
	const first = customEmoji.load();
	let secondResolved = false;
	const second = customEmoji.load().then(() => { secondResolved = true; });
	await nextTurn();
	const secondResolvedEarly = secondResolved;
	release();
	await Promise.all([first, second]);
	await customEmoji.draw(['sample']);
	console.log(JSON.stringify({ requests, secondResolvedEarly, image: customEmoji.nameToURL.get('sample') }));
} else if (scenario === 'conversation') {
	mock.module('$lib/messenger/cache-scope', () => ({ messengerCacheScope: async () => ({ key: 'scope', generation: 0 }), requireCurrentMessengerScope: async () => {} }));
	let releaseDirectory = () => {};
	let releaseEmoji = () => {};
	let postsStarted = false;
	let drawn = 0;
	const directory = { nameOfMember: new Map(), nameOfExternal: new Map(), memberOfExternal: new Map(), externalsOfMember: new Map(), memberOfEmail: new Map(), adminMemberIDs: new Set() };
	const people = new Promise<typeof directory>((resolve) => { releaseDirectory = () => resolve(directory); });
	const emoji = new Promise<void>((resolve) => { releaseEmoji = resolve; });
	mock.module('$lib/i18n/locale.svelte', () => ({ currentLocale: { value: 'en' } }));
	mock.module('$lib/person-name.svelte', () => ({ currentPersonNameLocale: () => 'en' }));
	mock.module('$lib/supabase-session', () => ({ supabaseMember: async () => ({ memberID: 'member' }) }));
	mock.module('$lib/stores/custom-emoji.svelte', () => ({ customEmoji: { load: () => emoji, draw: async () => { drawn += 1; }, nameToURL: new Map() } }));
	mock.module('$lib/stores/person-picture.svelte', () => ({ personPicture: { rememberExternals: async () => {}, rememberEveryone: async () => {} } }));
	mock.module('$lib/stores/attachment-source.svelte', () => ({ attachmentSource: { wants: async () => {}, openable: () => '' } }));
	mock.module('$lib/messenger/messenger-directory', () => ({
		fetchMessengerDirectory: () => people,
		externalIDsOfMember: () => [],
		personKey: () => 'member',
		personLabel: () => 'Sample'
	}));
	mock.module('$lib/messenger/messenger-api', () => ({
		addReaction: async () => {}, deletePost: async () => {}, editPost: async () => {},
		fetchChannels: async () => ({}), fetchPeople: async () => [], openDirectChannel: async () => ({}),
		removeReaction: async () => {}, writePost: async () => {}, keepAttachmentForSending: async () => ({}),
		fetchPosts: async () => {
			postsStarted = true;
			return [{ id: 'post', author: { memberID: 'member' }, body: 'Message text', postedAt: '2026-10-02', reactions: [], attachments: [] }];
		}
	}));
	const { bridgeConversation } = await import('../../src/lib/messenger/channel-over-bridge');
	const conversation = bridgeConversation('channel');
	await nextTurn();
	const postsBeforeDirectory = postsStarted;
	releaseDirectory();
	const result = await conversation;
	const emojiBefore = drawn;
	releaseEmoji();
	await nextTurn();
	console.log(JSON.stringify({ postsStarted: postsBeforeDirectory, text: result.messages[0]?.text, emojiBefore, emojiAfter: drawn }));
} else if (scenario === 'asset-scope') {
	Reflect.set(globalThis, '$state', <Value>(value: Value): Value => value);
	const resets: (() => void)[] = [];
	let releaseEmoji = () => {};
	let releaseAvatar = () => {};
	let releaseAttachment = () => {};
	let signedOldAssets = 0;
	let useNewEmoji = false;
	const emoji = new Promise<{ dataURL: string }>((resolve) => { releaseEmoji = () => resolve({ dataURL: 'old-image' }); });
	const avatar = new Promise<{ address: string }>((resolve) => { releaseAvatar = () => resolve({ address: 'old-avatar' }); });
	const attachment = new Promise<{ address: string }>((resolve) => { releaseAttachment = () => resolve({ address: 'old-address' }); });
	mock.module('$lib/messenger/cache-scope', () => ({ messengerCacheKey: () => 'scope', onMessengerCacheReset: (reset: () => void) => { resets.push(reset); return () => {}; } }));
	mock.module('$lib/supabase', () => ({ isSupabaseConfigured: () => true, projectURL: () => 'https://project.example.com', supabase: () => ({ storage: { from: () => ({}) } }) }));
	mock.module('$lib/messenger/messenger-api', () => ({
		fetchCustomEmojiNames: async () => ['logo'], fetchCustomEmojiImage: () => useNewEmoji ? Promise.resolve({ dataURL: 'new-image' }) : emoji,
		fetchPeople: async () => [{ externalID: 'same-person', avatarURL: 'source' }],
		keepPersonPictureForReading: () => avatar, copyAttachmentForReading: () => attachment
	}));
	mock.module('$lib/messenger/messenger-directory', () => ({ accountsHeldBy: () => [], fetchMessengerDirectory: async () => null }));
	mock.module('$lib/messenger/kept-attachment', () => ({
		assetBucket: 'assets', readableForSeconds: 86400, keptAssetPathOf: () => null,
		readableAddresses: async () => { signedOldAssets += 1; return new Map([['old-avatar', 'signed-old-avatar'], ['old-address', 'signed-old-file']]); }
	}));
	mock.module('$lib/transfer/company-transfer', () => ({ signedForReading: async () => { signedOldAssets += 1; return 'signed-old-file'; } }));
	const { customEmoji } = await import('../../src/lib/stores/custom-emoji.svelte');
	const { personPicture } = await import('../../src/lib/stores/person-picture.svelte');
	const { attachmentSource } = await import('../../src/lib/stores/attachment-source.svelte');
	await customEmoji.load();
	const drawing = customEmoji.draw(['logo']);
	const picturing = personPicture.rememberExternals(['same-person']);
	const signing = attachmentSource.wants([{ url: 'old-url', filename: 'old.png', contentType: 'image/png', sizeBytes: 1, digest: 'digest' }]);
	await nextTurn();
	for (const reset of resets) reset();
	releaseEmoji(); releaseAvatar(); releaseAttachment();
	await Promise.all([drawing, picturing, signing]);
	const oldEmoji = customEmoji.nameToURL.get('logo') ?? null;
	useNewEmoji = true;
	await customEmoji.load();
	await customEmoji.draw(['logo']);
	console.log(JSON.stringify({ emoji: oldEmoji, avatar: personPicture.pictureOfExternal('same-person'), attachment: attachmentSource.openable('old-url'), attachmentStatus: attachmentSource.status('old-url'), signedOldAssets, newEmoji: customEmoji.nameToURL.get('logo') }));
} else if (scenario === 'cache-scope') {
	const { Window } = await import('happy-dom');
	const browser = new Window();
	Reflect.set(globalThis, 'sessionStorage', browser.sessionStorage);
	Reflect.set(globalThis, 'localStorage', browser.localStorage);
	type Session = { access_token: string; user: { id: string } };
	let session: Session | null = { access_token: 'first-token', user: { id: 'first' } };
	let authChanged: (event: string, session: Session | null) => void = () => {};
	let company = 'first-company';
	let delayVerification: Promise<void> | null = null;
	mock.module('$lib/supabase', () => ({
		isSupabaseConfigured: () => true,
		projectURL: () => 'https://project.example.com',
		supabase: () => ({
			auth: { getSession: async () => ({ data: { session } }), onAuthStateChange: (listener: typeof authChanged) => { authChanged = listener; return {}; } },
			from: (table: string) => {
				const name = session?.user.id ?? '';
				const query = { select: () => query, neq: () => query, returns: async () => ({ error: null, data: table === 'member' ? [{ id: 'member', name, email: 'sample@example.com', messenger: null, is_admin: false }] : [] }) };
				return query;
			}
		})
	}));
	mock.module('$lib/company-session-scope', () => ({
		readVerifiedCompanyScope: async () => {
			const scope = { projectURL: 'https://project.example.com', accountID: session?.user.id ?? '', companyID: company, accessToken: session?.access_token ?? '', sessionKey: 'fingerprint' };
			if (delayVerification) await delayVerification;
			return scope;
		}
	}));
	const cache = await import('../../src/lib/messenger/cache-scope');
	const { fetchMessengerDirectory } = await import('../../src/lib/messenger/messenger-directory');
	const messages = await import('../../src/lib/components/channel/channel-message-cache');
	const reopenedKey = 'messenger-conversations:' + JSON.stringify(['https://project.example.com', 'first', 'first-company']);
	localStorage.setItem(reopenedKey, '["reopened-first"]');
	const firstScope = await cache.messengerCacheScope();
	const reopenPreserved = localStorage.getItem(reopenedKey) !== null;
	const firstDirectory = await fetchMessengerDirectory();
	const sameScopeReused = firstDirectory === await fetchMessengerDirectory();
	messages.setCachedMessages('shared-channel', []);
	const firstStorageKey = cache.conversationStorageKey(firstScope);
	localStorage.setItem(firstStorageKey, '["private-first"]');
	localStorage.setItem(`messenger-last-channel:${firstScope.key}`, 'first-channel');
	session = { access_token: 'first-refreshed', user: { id: 'first' } };
	authChanged('TOKEN_REFRESHED', session);
	await cache.messengerCacheScope();
	const refreshPreserved = localStorage.getItem(firstStorageKey) !== null && messages.getCachedMessages('shared-channel') !== undefined;
	session = { access_token: 'second-token', user: { id: 'second' } };
	company = 'second-company';
	authChanged('SIGNED_IN', session);
	const oldMessagesGone = messages.getCachedMessages('shared-channel') === undefined;
	const secondScope = await cache.messengerCacheScope();
	const secondDirectory = await fetchMessengerDirectory();
	const oldStoredGone = localStorage.getItem(firstStorageKey) === null;
	let releaseVerification = () => {};
	delayVerification = new Promise((resolve) => { releaseVerification = resolve; });
	cache.invalidateMessengerCacheScope();
	const late = cache.messengerCacheScope().then(() => false, () => true);
	await nextTurn();
	session = { access_token: 'third-token', user: { id: 'third' } };
	company = 'third-company';
	authChanged('SIGNED_IN', session);
	delayVerification = null;
	const thirdScope = await cache.messengerCacheScope();
	releaseVerification();
	const lateRefused = await late;
	const correctAfterRace = cache.messengerCacheKey() === thirdScope.key;
	localStorage.setItem(cache.conversationStorageKey(thirdScope), '[]');
	session = null;
	authChanged('SIGNED_OUT', session);
	console.log(JSON.stringify({ keysDiffer: firstScope.key !== secondScope.key, containsToken: firstScope.key.includes('first-token'),
		firstName: firstDirectory.nameOfMember.get('member'), secondName: secondDirectory.nameOfMember.get('member'), sameScopeReused, oldMessagesGone, oldStoredGone,
		lateRefused, correctAfterRace, reopenPreserved, refreshPreserved, logoutCleared: sessionStorage.length === 0 && localStorage.length === 0 && cache.messengerCacheKey() === '' }));
} else if (scenario === 'remembered-scope') {
	const { Window } = await import('happy-dom');
	const browser = new Window();
	Reflect.set(globalThis, 'sessionStorage', browser.sessionStorage);
	Reflect.set(globalThis, 'localStorage', browser.localStorage);
	type Session = { access_token: string; user: { id: string } };
	let session: Session | null = { access_token: 'other-token', user: { id: 'other' } };
	let authChanged: (event: string, session: Session | null) => void = () => {};
	let releaseVerification = () => {};
	const verification = new Promise<void>((resolve) => { releaseVerification = resolve; });
	mock.module('$lib/supabase', () => ({
		isSupabaseConfigured: () => true,
		projectURL: () => 'https://project.example.com',
		supabase: () => ({ auth: { getSession: async () => ({ data: { session } }), onAuthStateChange: (listener: typeof authChanged) => { authChanged = listener; return {}; } } })
	}));
	mock.module('$lib/company-session-scope', () => ({
		readVerifiedCompanyScope: async () => {
			await verification;
			return { projectURL: 'https://project.example.com', accountID: session?.user.id ?? '', companyID: 'company', accessToken: session?.access_token ?? '', sessionKey: 'fingerprint' };
		}
	}));
	const cache = await import('../../src/lib/messenger/cache-scope');
	const messages = await import('../../src/lib/components/channel/channel-message-cache');
	const { readCachedConversations } = await import('../../src/lib/messenger/conversation-list-cache');
	const key = JSON.stringify(['https://project.example.com', 'first', 'company']);
	const kept = { id: 'p1', sender: { id: 'member:m1', name: '이샘플' }, text: '안녕', sentAt: '2026-10-06T10:00:00Z' };
	localStorage.setItem('messenger-verified-scope', key);
	localStorage.setItem(cache.conversationStorageKey({ key, generation: 0 }), '[{"id":"kept-channel"}]');
	localStorage.setItem(cache.messageStorageKey(key, 'c1'), JSON.stringify([kept]));
	localStorage.setItem(cache.readerStorageKey(key), 'member:m1');
	const otherAccountAdopted = await cache.adoptRememberedMessengerScope();
	session = { access_token: 'first-token', user: { id: 'first' } };
	const adopted = await cache.adoptRememberedMessengerScope();
	const listBeforeVerification = adopted ? readCachedConversations(adopted).map((conversation) => conversation.id) : [];
	const messagesBeforeVerification = messages.getCachedMessages('c1')?.map((message) => message.text);
	const readerBeforeVerification = messages.getCachedReaderID();
	releaseVerification();
	await cache.messengerCacheScope();
	messages.setCachedMessages('c2', Array.from({ length: 60 }, (_, index) => ({ ...kept, id: `p${index}` })));
	const storedLength = JSON.parse(localStorage.getItem(cache.messageStorageKey(key, 'c2')) ?? '[]').length;
	session = null;
	authChanged('SIGNED_OUT', session);
	console.log(JSON.stringify({ otherAccountAdopted, adoptedKey: adopted?.key === key, listBeforeVerification, messagesBeforeVerification,
		readerBeforeVerification, storedLength, logoutCleared: localStorage.length === 0 && messages.getCachedMessages('c1') === undefined }));
} else if (scenario === 'ownership') {
	type Session = { access_token: string; user: { id: string } };
	let session: Session | null = { access_token: 'first-token', user: { id: 'first-account' } };
	let authChanged: (event: string, session: Session | null) => void = () => {};
	const sockets: OwnedSocket[] = [];
	class OwnedSocket {
		listeners = new Map<string, ((event: { data?: string }) => void)[]>();
		sent: string[] = [];
		closed = false;
		constructor() { sockets.push(this); }
		addEventListener(name: string, listener: (event: { data?: string }) => void) { this.listeners.set(name, [...(this.listeners.get(name) ?? []), listener]); }
		emit(name: string, event = {}) { for (const listener of this.listeners.get(name) ?? []) listener(event); }
		send(value: string) { this.sent.push(value); }
		close() { this.closed = true; this.emit('close'); }
		ready() { this.emit('open'); this.emit('message', { data: JSON.stringify({ kind: 'presence', isServerConnected: true }) }); }
		answer(requestID: string, status: number) { this.emit('message', { data: JSON.stringify({ kind: 'result', requestID, status, body: {} }) }); }
	}
	Reflect.set(globalThis, 'WebSocket', OwnedSocket);
	mock.module('$lib/supabase', () => ({
		gatewayURL: () => 'wss://gateway.example.com',
		supabase: () => ({ auth: {
			getSession: async () => ({ data: { session } }),
			onAuthStateChange: (listener: typeof authChanged) => { authChanged = listener; return {}; }
		} })
	}));
	let readMember = async () => ({ companyID: 'company' });
	mock.module('$lib/supabase-session', () => ({ supabaseMember: () => readMember() }));
	const { callCompanyApp } = await import('../../src/lib/host-bridge');
	const first = callCompanyApp({ capability: 'first' }).then(() => '', (error: Error) => error.name);
	await nextTurn();
	const firstSocket = sockets[0];
	if (!firstSocket) throw new Error('first socket missing');
	firstSocket.ready();
	await nextTurn();
	session = { access_token: 'second-token', user: { id: 'second-account' } };
	authChanged('SIGNED_IN', session);
	const oldFailure = await first;
	let secondFinished = false;
	const second = callCompanyApp({ capability: 'second' }).then((answer) => { secondFinished = true; return answer; });
	await nextTurn();
	const secondSocket = sockets[1];
	if (!secondSocket) throw new Error('second socket missing');
	secondSocket.ready();
	await nextTurn();
	const secondRequest = JSON.parse(secondSocket.sent[0] ?? '{}');
	firstSocket.answer(secondRequest.requestID, 599);
	await nextTurn();
	const acceptedOldResult = secondFinished;
	secondSocket.answer(secondRequest.requestID, 201);
	const secondStatus = (await second).status;
	const expectedOwnerRefused = await callCompanyApp({ capability: 'wrong-owner' }, {
		accountID: 'first-account', companyID: 'company', accessToken: 'first-token'
	}).then(() => false, () => true);
	session = { access_token: 'rotated-token', user: { id: 'second-account' } };
	authChanged('TOKEN_REFRESHED', session);
	const third = callCompanyApp({ capability: 'third' });
	await nextTurn();
	const thirdSocket = sockets[2];
	if (!thirdSocket) throw new Error('third socket missing');
	thirdSocket.ready();
	await nextTurn();
	thirdSocket.answer(JSON.parse(thirdSocket.sent[0] ?? '{}').requestID, 202);
	const thirdStatus = (await third).status;
	session = null;
	authChanged('SIGNED_OUT', session);
	const signOutRefused = await callCompanyApp({ capability: 'signed-out' }).then(() => false, () => true);
	let releaseMember = () => {};
	readMember = () => new Promise((resolve) => { releaseMember = () => resolve({ companyID: 'company' }); });
	session = { access_token: 'race-before', user: { id: 'race-before' } };
	const racing = callCompanyApp({ capability: 'race' }).then(() => false, () => true);
	await nextTurn();
	session = { access_token: 'race-after', user: { id: 'race-after' } };
	releaseMember();
	const raceRefused = await racing;
	console.log(JSON.stringify({ oldFailure, oldClosed: firstSocket.closed, acceptedOldResult, secondStatus,
		rotatedClosed: secondSocket.closed, thirdStatus, signOutClosed: thirdSocket.closed, signOutRefused, raceRefused, expectedOwnerRefused, connections: sockets.length }));
} else if (scenario === 'disconnect') {
	let socket: FakeSocket | undefined;
	class FakeSocket {
		listeners = new Map<string, ((event: { data?: string }) => void)[]>();
		sent: string[] = [];
		constructor() { socket = this; }
		addEventListener(name: string, listener: (event: { data?: string }) => void) {
			this.listeners.set(name, [...(this.listeners.get(name) ?? []), listener]);
		}
		emit(name: string, event = {}) { for (const listener of this.listeners.get(name) ?? []) listener(event); }
		send(value: string) { this.sent.push(value); }
		close() { this.emit('close'); }
	}
	Reflect.set(globalThis, 'WebSocket', FakeSocket);
	mock.module('$lib/supabase', () => ({
		gatewayURL: () => 'wss://gateway.example.com',
		supabase: () => ({ auth: {
			getSession: async () => ({ data: { session: { access_token: 'test-token', user: { id: 'account' } } } }),
			onAuthStateChange: () => ({})
		} })
	}));
	mock.module('$lib/supabase-session', () => ({ supabaseMember: async () => ({ companyID: 'company' }) }));
	const { callCompanyApp } = await import('../../src/lib/host-bridge');
	const calls = [callCompanyApp({ capability: 'first' }), callCompanyApp({ capability: 'second' })];
	const failures = Promise.all(calls.map((call) => call.then(() => 'unexpected success', (error: Error) => error.name)));
	await nextTurn();
	if (!socket) throw new Error('the wire was not opened');
	socket.emit('open');
	socket.emit('message', { data: JSON.stringify({ kind: 'presence', isServerConnected: true }) });
	await nextTurn();
	socket.close();
	console.log(JSON.stringify({ sent: socket.sent.length, failures: await failures }));
} else {
	throw new Error(`unknown scenario ${scenario}`);
}
