//   bun run web/scripts/check-attendance-reconcile.ts --url http://127.0.0.1:54321 --key <service role> --app http://localhost:5178

import { createClient } from '@supabase/supabase-js';
import { addMember, issueAgentKey, provisionCompany } from '../src/lib/server/control-plane';

function argument(name: string): string | undefined {
	const index = process.argv.indexOf(`--${name}`);
	return index >= 0 ? process.argv[index + 1] : undefined;
}

const projectURL = argument('url') ?? '';
const serviceRoleKey = argument('key') ?? '';
const appURL = argument('app') ?? 'http://localhost:5178';
if (!projectURL || !serviceRoleKey) throw new Error('pass --url and --key');

const admin = createClient(projectURL, serviceRoleKey, {
	auth: { autoRefreshToken: false, persistSession: false }
});

const stamp = crypto.randomUUID().slice(0, 8);
const from = '2026-08-01T00:00:00.000Z';
const to = '2026-09-01T00:00:00.000Z';
const firstExternalID = `U-first-${stamp}`;
const secondExternalID = `U-second-${stamp}`;

const company = await provisionCompany(
	admin,
	{ name: 'Reconcile check', slug: `reconcile-${stamp}`, country: 'KR', locale: 'ko', timezone: 'Asia/Seoul' },
	`first-${stamp}@example.test`
);
const findings: [string, boolean][] = [];

async function reconcile(agentKey: string, events: unknown[]): Promise<Record<string, unknown>> {
	const response = await fetch(`${appURL}/api/agent/attendance-reconcile`, {
		method: 'POST',
		headers: { Authorization: `Bearer ${agentKey}`, 'Content-Type': 'application/json' },
		body: JSON.stringify({ platform: 'mattermost', from, to, events })
	});
	return (await response.json()) as Record<string, unknown>;
}

async function heldFor(memberID: string): Promise<{ kind: string; occurred_at: string }[]> {
	const { data } = await admin
		.from('attendance')
		.select('kind, occurred_at')
		.eq('member_id', memberID)
		.gte('occurred_at', from)
		.lt('occurred_at', to)
		.order('occurred_at')
		.returns<{ kind: string; occurred_at: string }[]>();
	return data ?? [];
}

try {
	const secondMemberID = await addMember(admin, company.companyID, `second-${stamp}@example.test`);
	const agent = await issueAgentKey(admin, company.companyID, `reconcile-${stamp}`);
	await admin.from('member').update({ messenger: { mattermost: firstExternalID } }).eq('id', company.adminMemberID);
	await admin.from('member').update({ messenger: { mattermost: secondExternalID } }).eq('id', secondMemberID);

	await admin.from('attendance').insert([
		{ member_id: company.adminMemberID, kind: 'clock_in', occurred_at: '2026-08-05T08:29:00Z' },
		{ member_id: company.adminMemberID, kind: 'clock_out', occurred_at: '2026-08-05T08:29:00Z' },
		{ member_id: secondMemberID, kind: 'clock_in', occurred_at: '2026-08-06T00:10:00Z' }
	]);

	const first = await reconcile(agent.apiKey, [
		{ externalID: firstExternalID, kind: 'clock_in', occurredAt: '2026-08-07T03:56:00Z', location: '본사' },
		{ externalID: firstExternalID, kind: 'clock_out', occurredAt: '2026-08-07T09:54:00Z', location: '본사' },
		{ externalID: secondExternalID, kind: 'clock_in', occurredAt: '2026-08-06T00:10:00Z', location: '' }
	]);

	findings.push(['the device is the one that decides', first.added === 2 && first.removed === 2]);
	findings.push([
		'a clock-out the device placed somewhere is stored without a place, which the record requires',
		(await heldFor(company.adminMemberID)).length === 2
	]);
	const mine = await heldFor(company.adminMemberID);
	findings.push([
		'a clock-in cancelled on the device stops being in the record',
		mine.length === 2 && mine[0].occurred_at.startsWith('2026-08-07T03:56')
	]);
	const theirs = await heldFor(secondMemberID);
	findings.push(['a colleague already right is left alone', theirs.length === 1]);

	const again = await reconcile(agent.apiKey, [
		{ externalID: firstExternalID, kind: 'clock_in', occurredAt: '2026-08-07T03:56:00Z', location: '본사' },
		{ externalID: firstExternalID, kind: 'clock_out', occurredAt: '2026-08-07T09:54:00Z', location: '본사' },
		{ externalID: secondExternalID, kind: 'clock_in', occurredAt: '2026-08-06T00:10:00Z', location: '' }
	]);
	findings.push(['running it twice changes nothing', again.added === 0 && again.removed === 0]);

	const emptied = await reconcile(agent.apiKey, []);
	findings.push([
		'a device reporting nothing is refused rather than obeyed',
		Array.isArray(emptied.refused) && (emptied.refused as string[]).length === 2 && emptied.removed === 0
	]);
	findings.push(['and the record still holds what it held', (await heldFor(company.adminMemberID)).length === 2]);

	const outside = await reconcile(agent.apiKey, [
		{ externalID: firstExternalID, kind: 'clock_in', occurredAt: '2026-08-07T03:56:00Z', location: '본사' },
		{ externalID: firstExternalID, kind: 'clock_out', occurredAt: '2026-08-07T09:54:00Z', location: '본사' },
		{ externalID: firstExternalID, kind: 'clock_in', occurredAt: '2026-09-02T01:00:00Z', location: '본사' },
		{ externalID: secondExternalID, kind: 'clock_in', occurredAt: '2026-08-06T00:10:00Z', location: '' }
	]);
	findings.push([
		'an event outside the window is not this window to reconcile',
		outside.added === 0 && outside.removed === 0 && (outside.rejected as string[]).length === 0
	]);

	const stranger = await reconcile('not-an-agent-key', []);
	findings.push(['a key belonging to no agent changes nothing', stranger.added === undefined]);
} finally {
	await admin.from('company').delete().eq('id', company.companyID);
	const accounts = await admin.auth.admin.listUsers();
	for (const account of accounts.data.users) {
		if (account.email?.includes(stamp)) await admin.auth.admin.deleteUser(account.id);
	}
}

for (const [what, held] of findings) console.log(`${held ? 'ok  ' : 'FAIL'} ${what}`);
if (findings.some(([, held]) => !held)) process.exit(1);
