//   bun run host/messenger/check-messenger-identity.ts --external-id <messenger user id>

import { createClient } from '@supabase/supabase-js';

function argument(name: string): string | undefined {
	const index = process.argv.indexOf(`--${name}`);
	return index >= 0 ? process.argv[index + 1] : undefined;
}

function required(name: string): string {
	const value = process.env[name];
	if (!value) throw new Error(`set ${name}`);
	return value;
}

const projectURL = required('SUPABASE_URL');
const publishableKey = required('SUPABASE_PUBLISHABLE_KEY');
const agentKey = required('AGENT_API_KEY');
const appURL = required('INTERNKIM_APP_URL');
const externalID = argument('external-id');
const kind = argument('kind') ?? 'mattermost';
if (!externalID) throw new Error('pass --external-id <messenger user id>');

const response = await fetch(`${appURL}/api/agent/session`, {
	method: 'POST',
	headers: { Authorization: `Bearer ${agentKey}`, 'Content-Type': 'application/json' },
	body: JSON.stringify({ kind, externalID })
});
if (!response.ok) throw new Error(`the plane refused this identity (${response.status})`);
const session = (await response.json()) as { memberID: string; accessToken: string };

const client = createClient(projectURL, publishableKey, {
	auth: { autoRefreshToken: false, persistSession: false },
	global: { headers: { Authorization: `Bearer ${session.accessToken}` } }
});

const me = await client
	.from('member')
	.select('name, email, job_title')
	.eq('id', session.memberID)
	.single<{ name: string | null; email: string | null; job_title: string | null }>();
if (me.error) throw new Error(`reading their own member failed: ${me.error.message}`);
console.log(`  ✓ acts as ${me.data.name} (${me.data.email}, ${me.data.job_title})`);

const attendance = await client
	.from('attendance')
	.select('kind, occurred_at')
	.eq('member_id', session.memberID)
	.order('occurred_at', { ascending: false })
	.limit(1);
if (attendance.error) throw new Error(`reading their attendance failed: ${attendance.error.message}`);
console.log(`  ✓ reads their own attendance (${attendance.data.length} recent)`);

const written = await client
	.from('attendance')
	.insert({ member_id: session.memberID, kind: 'clock_in' })
	.select('id, location')
	.single<{ id: string; location: string | null }>();
if (written.error) {
	console.log(`  · clock-in refused: ${written.error.message.split('\n')[0]}`);
} else {
	console.log(`  ✓ clocks in at ${written.data.location}`);
	await client.from('attendance').delete().eq('id', written.data.id);
	console.log('  ✓ removed the probe row');
}

const colleagues = await client.from('member').select('id');
if (colleagues.error) throw new Error(colleagues.error.message);
console.log(`  ✓ sees ${colleagues.data.length} colleagues, and no other company`);

const others = await client.from('company').select('id');
if (others.error) throw new Error(others.error.message);
console.log(`  ✓ sees ${others.data.length} company`);

console.log('\nno web sign-in was used');
