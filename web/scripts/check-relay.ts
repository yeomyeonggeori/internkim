//   bun run web/scripts/check-relay.ts --url http://127.0.0.1:54321 --key <service role> --publishable <publishable key> --app http://localhost:5178 --relay ./internkim-relay

import { createClient } from '@supabase/supabase-js';
import { addMember, issueAgentKey, provisionCompany } from '../src/lib/server/control-plane';
import { saveCompanyConnection } from '../src/lib/server/company-credential';
import { aBrowserThatSubscribed } from '../tests/support/read-as-the-browser-would';

function argument(name: string): string | undefined {
	const index = process.argv.indexOf(`--${name}`);
	return index >= 0 ? process.argv[index + 1] : undefined;
}

const projectURL = argument('url') ?? '';
const serviceRoleKey = argument('key') ?? '';
const publishableKey = argument('publishable') ?? '';
const appURL = argument('app') ?? 'http://localhost:5178';
const relayPath = argument('relay') ?? './internkim-relay';
const arrivalsPort = 18094;
if (!projectURL || !serviceRoleKey || !publishableKey) throw new Error('pass --url, --key and --publishable');

const admin = createClient(projectURL, serviceRoleKey, {
	auth: { autoRefreshToken: false, persistSession: false }
});

const stamp = crypto.randomUUID().slice(0, 8);
const botToken = `bot-token-${stamp}`;
const firstExternalID = `U-first-${stamp}`;
const secondExternalID = `U-second-${stamp}`;

function aMessengerNobodyRuns(port: number) {
	const minted: string[] = [];
	let refusesLogin = false;
	let loginAttempts = 0;
	const server = Bun.serve({
		port,
		fetch: async (request) => {
			const path = new URL(request.url).pathname;
			if (path === '/api/v4/users/login') {
				loginAttempts += 1;
				if (refusesLogin) {
					return Response.json({ message: 'that password is wrong' }, { status: 401 });
				}
				return new Response(JSON.stringify({ id: `bot-${stamp}` }), {
					headers: { Token: botToken, 'Content-Type': 'application/json' }
				});
			}
			if (path === '/api/v4/users') {
				return Response.json([
					{ id: firstExternalID, username: 'first', first_name: '이', last_name: '샘플', email: `first-${stamp}@example.test` },
					{ id: secondExternalID, username: 'second', first_name: '박', last_name: '예시', email: `second-${stamp}@example.test` }
				]);
			}
			if (path.endsWith('/tokens')) {
				const userID = path.split('/')[4];
				minted.push(userID);
				return Response.json({ token: `personal-${userID}` });
			}
			return new Response('{}', { headers: { 'Content-Type': 'application/json' } });
		}
	});
	return {
		url: `http://127.0.0.1:${server.port}`,
		minted,
		stop: () => server.stop(true),
		refuseTheAccount: () => {
			refusesLogin = true;
			loginAttempts = 0;
		},
		loginAttempts: () => loginAttempts
	};
}

function aConnectorNobodyRuns(port: number) {
	const asked: { capability: string; actorSecret: string }[] = [];
	const server = Bun.serve({
		port,
		fetch: async (request) => {
			const capability = decodeURIComponent(new URL(request.url).pathname.split('/').pop() ?? '');
			const body = (await request.json()) as { actor?: { secret?: string } };
			asked.push({ capability, actorSecret: body.actor?.secret ?? '' });
			if (capability === 'person.identity') {
				const externalID = body.actor?.secret === `personal-${firstExternalID}` ? firstExternalID : secondExternalID;
				return Response.json({ externalID });
			}
			return Response.json({ conversations: [{ id: `channel-for-${body.actor?.secret ?? 'nobody'}` }] });
		}
	});
	return { url: `http://127.0.0.1:${server.port}`, asked, stop: () => server.stop(true) };
}

async function signUp(email: string, memberID: string): Promise<string> {
	const account = await admin.auth.admin.createUser({ email, password: 'seed-password', email_confirm: true });
	if (account.error) throw new Error(`${email}: ${account.error.message}`);
	const linked = await admin.from('member').update({ user_id: account.data.user.id }).eq('id', memberID);
	if (linked.error) throw new Error(`${email}: ${linked.error.message}`);
	return account.data.user.id;
}

async function untilTrue(what: string, ready: () => Promise<boolean>, seconds: number): Promise<boolean> {
	for (let attempt = 0; attempt < seconds * 2; attempt += 1) {
		if (await ready()) return true;
		await Bun.sleep(500);
	}
	console.error(`gave up waiting for ${what}`);
	return false;
}

const company = await provisionCompany(
	admin,
	{ name: 'Relay check', slug: `relay-check-${stamp}`, country: 'KR', locale: 'ko', timezone: 'Asia/Seoul' },
	`first-${stamp}@example.test`
);

const messenger = aMessengerNobodyRuns(0);
const connector = aConnectorNobodyRuns(0);
const findings: [string, boolean][] = [];
let relay: Bun.Subprocess | undefined;

try {
	const secondMemberID = await addMember(admin, company.companyID, `second-${stamp}@example.test`);
	await signUp(`first-${stamp}@example.test`, company.adminMemberID);
	await signUp(`second-${stamp}@example.test`, secondMemberID);
	await saveCompanyConnection(admin, company.companyID, {
		kind: 'mattermost',
		host: messenger.url,
		settings: { username: `bot-${stamp}@example.test` },
		secret: 'bot-password'
	});
	const agent = await issueAgentKey(admin, company.companyID, `relay-check-${stamp}`);

	const relayEnvironment = {
		...process.env,
		SUPABASE_URL: projectURL,
		SUPABASE_PUBLISHABLE_KEY: publishableKey,
		INTERNKIM_APP_URL: appURL,
		MESSENGER_PLATFORM: 'mattermost',
		CHATD_BASE_URL: connector.url,
		ARRIVALS_PORT: String(arrivalsPort),
		AGENT_API_KEY: agent.apiKey
	};
	relay = Bun.spawn([relayPath], { env: relayEnvironment, stdout: 'inherit', stderr: 'inherit' });

	const joined = await untilTrue(
		'the relay to record its contacts',
		async () => {
			const contacts = await admin
				.from('contact')
				.select('external_id')
				.eq('company_id', company.companyID)
				.returns<{ external_id: string }[]>();
			return (contacts.data ?? []).length === 2;
		},
		30
	);
	findings.push(['the relay starts and records who the messenger knows', joined]);
	findings.push([
		'it mints a token for each of them',
		messenger.minted.includes(firstExternalID) && messenger.minted.includes(secondExternalID)
	]);

	const asMember = createClient(projectURL, publishableKey, {
		auth: { autoRefreshToken: false, persistSession: false }
	});
	const signedIn = await asMember.auth.signInWithPassword({
		email: `first-${stamp}@example.test`,
		password: 'seed-password'
	});
	if (signedIn.error) throw new Error(signedIn.error.message);
	await asMember.realtime.setAuth(signedIn.data.session?.access_token ?? '');

	const answers = asMember.channel(`member:${company.adminMemberID}`, { config: { private: true } });
	const heard: { callID?: string; status?: number; body?: unknown }[] = [];
	answers.on('broadcast', { event: 'answer' }, ({ payload }) => heard.push(payload));
	await new Promise<void>((resolve, reject) => {
		answers.subscribe((status, error) => {
			if (status === 'SUBSCRIBED') resolve();
			if (error) reject(error);
		});
	});

	const listening = await untilTrue(
		'the relay to appear on the company channel',
		async () => {
			const presence = asMember.channel(`company:${company.companyID}`, { config: { private: true } });
			const seen = await new Promise<boolean>((resolve) => {
				presence.on('presence', { event: 'sync' }, () => resolve(Object.keys(presence.presenceState()).length > 0));
				presence.subscribe();
				setTimeout(() => resolve(false), 3000);
			});
			await asMember.removeChannel(presence);
			return seen;
		},
		30
	);
	findings.push(['a member can see that the app is running', listening]);

	const callID = crypto.randomUUID();
	await answers.send({
		type: 'broadcast',
		event: 'call',
		payload: {
			callID,
			capability: 'person.conversations.list',
			replyTo: company.adminMemberID,
			body: { actor: { kind: 'mattermost-token', secret: `personal-${firstExternalID}` } }
		}
	});

	const answered = await untilTrue('an answer', async () => heard.some((entry) => entry.callID === callID), 20);
	findings.push(['a call travels browser to relay to connector and back', answered]);
	const answer = heard.find((entry) => entry.callID === callID);
	findings.push(['the answer is the caller’s own', answer?.status === 200]);
	findings.push([
		'the connector was asked as the caller, not as the bot',
		connector.asked.some(
			(entry) => entry.capability === 'person.conversations.list' && entry.actorSecret === `personal-${firstExternalID}`
		)
	]);
	findings.push([
		'the relay proved who the caller was before doing the work',
		connector.asked[0]?.capability === 'person.identity'
	]);

	const subscriber = await aBrowserThatSubscribed();
	const pushService = Bun.serve({
		port: 0,
		fetch: () => new Response(null, { status: 201 })
	});
	await admin.from('push_device').insert({
		member_id: company.adminMemberID,
		kind: 'web-push',
		address: `http://127.0.0.1:${pushService.port}/push`,
		keys: subscriber.keys
	});
	const arrival = await fetch(`http://127.0.0.1:${arrivalsPort}`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({
			conversationID: 'channel-1',
			messageID: `post-${stamp}`,
			authorExternalID: secondExternalID,
			recipientExternalIDs: [firstExternalID, secondExternalID],
			preview: '오늘 회의 30분 미뤄도 될까요'
		})
	});
	const reported = (await arrival.json()) as { told?: number };
	findings.push(['a message the messenger accepted becomes a notification', reported.told === 1]);
	pushService.stop(true);

	relay.kill();
	await relay.exited;
	messenger.refuseTheAccount();
	relay = Bun.spawn([relayPath], { env: relayEnvironment, stdout: 'inherit', stderr: 'inherit' });


	const askedBeforeTheRefusal = connector.asked.length;
	const servedAnyway = await untilTrue(
		'the relay to take a call while the messenger refuses the account it was given',
		async () => {
			await answers.send({
				type: 'broadcast',
				event: 'call',
				payload: {
					callID: crypto.randomUUID(),
					capability: 'person.conversations.list',
					replyTo: company.adminMemberID,
					body: { actor: { kind: 'mattermost-token', secret: `personal-${firstExternalID}` } }
				}
			});
			return connector.asked.length > askedBeforeTheRefusal;
		},
		30
	);
	findings.push(['the relay still serves calls when the messenger refuses its account', servedAnyway]);
	findings.push([
		'the relay never signs in to the messenger at all, so no account of anyone else can be locked',
		messenger.loginAttempts() === 0
	]);

	await asMember.removeAllChannels();
	asMember.realtime.disconnect();
} finally {
	relay?.kill();
	messenger.stop();
	connector.stop();
	await admin.from('company').delete().eq('id', company.companyID);
	const accounts = await admin.auth.admin.listUsers();
	for (const account of accounts.data.users) {
		if (account.email?.includes(stamp)) await admin.auth.admin.deleteUser(account.id);
	}
}

for (const [what, held] of findings) console.log(`${held ? 'ok  ' : 'FAIL'} ${what}`);
if (findings.some(([, held]) => !held)) process.exit(1);
