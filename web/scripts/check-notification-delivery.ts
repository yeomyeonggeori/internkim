//   bun run web/scripts/check-notification-delivery.ts --url http://127.0.0.1:54321 --key <service role> --app http://127.0.0.1:5173

import { createClient } from '@supabase/supabase-js';
import { issueAgentKey, provisionCompany } from '../src/lib/server/control-plane';
import { aBrowserThatSubscribed, readAsTheBrowserWould } from '../tests/support/read-as-the-browser-would';

function argument(name: string): string | undefined {
	const index = process.argv.indexOf(`--${name}`);
	return index >= 0 ? process.argv[index + 1] : undefined;
}

const projectURL = argument('url') ?? process.env.SUPABASE_URL ?? '';
const serviceRoleKey = argument('key') ?? process.env.SUPABASE_SECRET_KEY ?? '';
const appURL = argument('app') ?? 'http://127.0.0.1:5173';
if (!projectURL || !serviceRoleKey) throw new Error('pass --url and --key');

const admin = createClient(projectURL, serviceRoleKey, {
	auth: { autoRefreshToken: false, persistSession: false }
});

type Arrival = { headers: Headers; body: Uint8Array<ArrayBuffer> };

async function aPushServiceThatRecords(status: number): Promise<{
	address: string;
	arrivals: Arrival[];
	stop: () => void;
}> {
	const arrivals: Arrival[] = [];
	const server = Bun.serve({
		port: 0,
		fetch: async (request) => {
			arrivals.push({
				headers: request.headers,
				body: new Uint8Array(await request.arrayBuffer())
			});
			return new Response(null, { status });
		}
	});
	return { address: `http://127.0.0.1:${server.port}/push`, arrivals, stop: () => server.stop(true) };
}

async function notify(agentKey: string, body: Record<string, unknown>): Promise<Response> {
	return fetch(`${appURL}/api/agent/notify`, {
		method: 'POST',
		headers: { Authorization: `Bearer ${agentKey}`, 'Content-Type': 'application/json' },
		body: JSON.stringify(body)
	});
}

const stamp = crypto.randomUUID().slice(0, 8);
const company = await provisionCompany(
	admin,
	{ name: 'Notify check', slug: `notify-check-${stamp}`, country: 'KR', locale: 'ko', timezone: 'Asia/Seoul' },
	`first-${stamp}@example.test`
);
const findings: [string, boolean][] = [];

try {
	const agent = await issueAgentKey(admin, company.companyID, `notify-check-${stamp}`);
	const agentKey = agent.apiKey;
	const externalID = `U-${stamp}`;
	await admin.from('contact').insert({
		company_id: company.companyID,
		platform: 'mattermost',
		external_id: externalID,
		name: '이샘플',
		member_id: company.adminMemberID
	});

	const subscriber = await aBrowserThatSubscribed();
	const pushService = await aPushServiceThatRecords(201);
	await admin.from('push_device').insert({
		member_id: company.adminMemberID,
		kind: 'web-push',
		address: pushService.address,
		keys: subscriber.keys
	});

	const sent = { title: '이샘플', body: '오늘 회의 30분 미뤄도 될까요', openPath: '/flow/', tag: `message:${stamp}` };
	const answered = await notify(agentKey, {
		platform: 'mattermost',
		externalIDs: [externalID],
		category: 'message',
		...sent
	});
	const report = (await answered.json()) as { told?: number; reached?: number; pruned?: number };

	findings.push(['the route accepts a call the host would make', answered.status === 200]);
	findings.push(['one member was told, on one device', report.told === 1 && report.reached === 1]);
	findings.push(['the push service received exactly one request', pushService.arrivals.length === 1]);

	const arrival = pushService.arrivals[0];
	findings.push([
		'it carries the headers a push service refuses without',
		arrival?.headers.get('content-encoding') === 'aes128gcm' &&
			(arrival?.headers.get('authorization') ?? '').startsWith('vapid t=')
	]);
	findings.push([
		'nothing readable crossed the wire',
		!new TextDecoder().decode(arrival?.body ?? new Uint8Array()).includes('회의')
	]);
	findings.push([
		'the browser that subscribed reads back what was sent',
		arrival ? (await readAsTheBrowserWould(arrival.body, subscriber)) === JSON.stringify(sent) : false
	]);
	pushService.stop();

	const quiet = await aPushServiceThatRecords(201);
	await admin
		.from('push_device')
		.update({ address: quiet.address })
		.eq('member_id', company.adminMemberID);
	await admin
		.from('member')
		.update({ notification_settings: { message: false } })
		.eq('id', company.adminMemberID);
	const silenced = await notify(agentKey, {
		platform: 'mattermost',
		externalIDs: [externalID],
		category: 'message',
		...sent
	});
	const silentReport = (await silenced.json()) as { told?: number };
	findings.push([
		'turning messages off in settings stops the push',
		silentReport.told === 0 && quiet.arrivals.length === 0
	]);
	quiet.stop();

	const forgotten = await aPushServiceThatRecords(410);
	await admin.from('push_device').update({ address: forgotten.address }).eq('member_id', company.adminMemberID);
	await admin
		.from('member')
		.update({ notification_settings: {} })
		.eq('id', company.adminMemberID);
	await notify(agentKey, { platform: 'mattermost', externalIDs: [externalID], category: 'message', ...sent });
	const surviving = await admin
		.from('push_device')
		.select('address')
		.eq('member_id', company.adminMemberID)
		.returns<{ address: string }[]>();
	findings.push(['a browser the push service has forgotten is pruned', (surviving.data ?? []).length === 0]);
	forgotten.stop();

	const strangerKey = 'not-an-agent-key';
	const refused = await notify(strangerKey, {
		platform: 'mattermost',
		externalIDs: [externalID],
		category: 'message',
		...sent
	});
	findings.push(['a key that belongs to no agent is refused', refused.status === 403]);
} finally {
	await admin.from('company').delete().eq('id', company.companyID);
	const accounts = await admin.auth.admin.listUsers();
	for (const account of accounts.data.users) {
		if (account.email?.includes(stamp)) await admin.auth.admin.deleteUser(account.id);
	}
}

for (const [what, held] of findings) console.log(`${held ? 'ok  ' : 'FAIL'} ${what}`);
if (findings.some(([, held]) => !held)) process.exit(1);
