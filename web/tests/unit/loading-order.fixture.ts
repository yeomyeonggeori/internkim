import { mock } from 'bun:test';

const scenario = process.argv[2];
const turn = () => new Promise<void>((resolve) => setTimeout(resolve, 0));

if (scenario === 'mail') {
	Reflect.set(globalThis, '$state', <Value>(value: Value): Value => value);
	const pages: string[] = [];
	const bodies: number[] = [];
	let releaseLate = () => {};
	let holdPage = false;
	let missing = false;
	const late = new Promise<{ messages: []; nextCursor: string }>((resolve) => { releaseLate = () => resolve({ messages: [], nextCursor: '' }); });
	mock.module('$lib/supabase', () => ({ isSupabaseConfigured: () => true }));
	mock.module('../../src/routes/mail/mail-account-api', () => ({ recordMailAccount: async () => ({ email: 'sample@example.com', isConfigured: true, defaultMailbox: 'INBOX' }), keepRecordMailAccount: async () => ({}), testRecordMailAccount: async () => ({}) }));
	mock.module('$lib/public-api-call', () => ({ invokeTool: async (name: string, input: Record<string, unknown>) => {
		if (name === 'mail_mailbox_list') return { mailboxes: [] };
		if (name === 'mail_message_list') {
			pages.push(`${input.mailbox}:${input.cursor ?? ''}`);
			if (holdPage) return late;
			if (missing) return { messages: [], nextCursor: '' };
			const identifiers = input.cursor ? [3] : [1, 2];
			return { messages: identifiers.map((uid) => ({ uid, mailbox: input.mailbox, subject: `Message ${uid}`, isRead: true })), nextCursor: input.cursor ? '' : 'later' };
		}
		if (name === 'mail_message_read') {
			const uid = Number(input.uid);
			bodies.push(uid);
			if (uid === 1) return new Promise(() => {});
			return { uid, mailbox: input.mailbox, subject: `Message ${uid}`, bodyText: `Body ${uid}` };
		}
		return {};
	} }));
	const { createMailPageController } = await import('../../src/routes/mail/mail-page-controller.svelte');
	const { mailText } = await import('../../src/routes/mail/text');
	const controller = createMailPageController(mailText.en);
	controller.requestMailboxMessage({ mailbox: 'Archive', uid: 2 });
	await controller.loadMail();
	await turn();
	const cold = { pages: [...pages], bodies: [...bodies], selected: controller.selectedMessage?.uid };
	pages.length = 0; bodies.length = 0;
	await controller.openMailboxMessage('Archive', 3);
	await turn();
	const later = { pages: [...pages], bodies: [...bodies], selected: controller.selectedMessage?.uid };
	missing = true;
	await controller.openMailboxMessage('Empty', 99);
	const reportedMissing = controller.errorMessage === mailText.en.errors.messageNotFound;
	const previousCount = pages.length;
	await controller.loadMail();
	const retried = pages.length > previousCount;
	holdPage = true;
	const reading = controller.openMailboxMessage('Late', 2);
	await turn();
	controller.account = { ...controller.account, email: 'another@example.com' };
	controller.selectMailbox('New');
	releaseLate();
	await reading;
	console.log(JSON.stringify({ cold, later, missing: reportedMissing, retried, lateIgnored: controller.selectedMessage === null && controller.selectedMailbox === 'New' }));
} else if (scenario === 'approval') {
	const { defaultLeavePolicy } = await import('../../src/lib/attendance/leave-policy-defaults');
	const policy = defaultLeavePolicy();
	const annual = policy.leaveTypes[0];
	if (!annual) throw new Error('default annual leave type missing');
	policy.leaveTypes.push({ ...annual, id: 'retired-kind', name: 'Historical', isActive: false });
	const started: string[] = [];
	let listInput: Record<string, unknown> = {};
	let releasePolicy = () => {};
	let refuse = false;
	const gate = new Promise<void>((resolve) => { releasePolicy = resolve; });
	mock.module('$lib/public-api-call', () => ({ ToolRefused: class extends Error {}, invokeTool: async (name: string, input: Record<string, unknown>) => {
		started.push(name);
		if (name === 'attendance_leave_policy_get') { await gate; return policy; }
		if (name === 'company_settings_get') return { timeZone: 'Asia/Seoul' };
		if (name === 'person_list') return { requesterID: 'member', count: 1, people: [{ personID: 'member', name: '이샘플', email: 'sample@example.com' }] };
		if (name === 'leave_list') {
			listInput = input;
			if (refuse) throw new Error('not allowed');
			return { count: 4, registeredKinds: [], leave: [
				['later', 'requested', '2026-10-05'], ['approved', 'approved', '2026-10-01'], ['earlier', 'requested', '2026-10-02'], ['rejected', 'rejected', '2026-10-03']
			].map(([leaveID, status, day]) => ({ leaveID, personID: 'member', kindID: 'retired-kind', kindName: 'Historical', isPaid: true, isDeducted: false, days: 1, status, startsAt: `${day}T00:00:00Z`, endsAt: `${day}T23:59:59Z`, note: '' })) };
		}
		throw new Error(name);
	} }));
	const { supabaseLeaveApprovalInbox } = await import('../../src/lib/attendance/supabase-leave');
	const reading = supabaseLeaveApprovalInbox();
	await turn();
	const before = [...started];
	releasePolicy();
	const result = await reading;
	refuse = true;
	let refused = false;
	try { await supabaseLeaveApprovalInbox(); } catch { refused = true; }
	console.log(JSON.stringify({ started: before, input: listInput, pending: result.pending.map((request) => request.id), historicalLabel: result.pending.every((request) => request.leaveTypeName === 'Historical'), refused }));
} else if (scenario === 'notifications') {
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
		build.onLoad({ filter: /\.svelte\.(js|ts)$/ }, ({ path }) => ({ contents: compileModule(new Bun.Transpiler({ loader: path.endsWith('.ts') ? 'ts' : 'js' }).transformSync(readFileSync(path, 'utf8')), { filename: path, generate: 'client' }).js.code, loader: 'js' }));
	} });
	let releaseReach = () => {};
	const gate = new Promise<'on'>((resolve) => { releaseReach = () => resolve('on'); });
	let reach: () => Promise<string> = () => gate;
	let preferencesStarted = false;
	let failPreferences = false;
	let failures = 0;
	mock.module('$lib/i18n/page-text.svelte', () => ({ createPageText: () => ({ notifications: 'Notifications', notifyMessage: 'Messages', notifyLoadFailed: 'Load failed' }) }));
	mock.module('$lib/notifications/subscribe', () => ({ reachability: () => reach(), startBeingReached: async () => 'on', stopBeingReached: async () => 'off' }));
	mock.module('$lib/notifications/settings', () => ({ myNotificationSettings: async () => { preferencesStarted = true; if (failPreferences) throw new Error('failed'); return { categories: [{ category: 'message', isOn: true, isChoosable: true }], mutedConversationIDs: [] }; }, chooseNotificationCategory: async () => ({}) }));
	mock.module('$lib/notifications/self-test', () => ({ sendTestNotification: async () => ({ reached: 0 }) }));
	mock.module('svelte-sonner', () => ({ toast: { error: () => { failures += 1; } } }));
	const { flushSync, mount, unmount } = await import('svelte');
	const { default: Notifications } = await import('../../src/routes/settings/notifications.svelte');
	const target = document.createElement('div');
	document.body.appendChild(target);
	const component = mount(Notifications, { target });
	flushSync(); await turn(); flushSync();
	const visible = target.textContent?.includes('Messages');
	const pendingDisabled = target.querySelector('[role=switch]')?.hasAttribute('disabled');
	releaseReach(); await turn(); flushSync();
	const onEnabled = !target.querySelector('[role=switch]')?.hasAttribute('disabled');
	await unmount(component);
	let refusedDisabled = true;
	for (const refused of ['blocked', 'unsupported', 'unconfigured']) {
		reach = async () => refused;
		const next = mount(Notifications, { target });
		flushSync(); await turn(); flushSync();
		refusedDisabled &&= Boolean(target.querySelector('[role=switch]')?.hasAttribute('disabled'));
		await unmount(next);
	}
	failPreferences = true;
	const failed = mount(Notifications, { target });
	flushSync(); await turn(); flushSync();
	await unmount(failed);
	console.log(JSON.stringify({ preferencesStarted, visible, pendingDisabled, onEnabled, refusedDisabled, loadFailure: failures === 1 }));
}
