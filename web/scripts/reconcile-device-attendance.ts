//   bun run web/scripts/reconcile-device-attendance.ts --device https://<device> --url <project> --key <service role> --app https://api.intern.kim --months 2026-05,2026-06 [--apply]

import { createClient } from '@supabase/supabase-js';
import { windowOf } from './attendance-window';
import { issueAgentKey, revokeAgent } from '../src/lib/server/control-plane';

function argument(name: string): string | undefined {
	const index = process.argv.indexOf(`--${name}`);
	return index >= 0 ? process.argv[index + 1] : undefined;
}

const deviceURL = (argument('device') ?? '').replace(/\/$/, '');
const projectURL = argument('url') ?? '';
const serviceRoleKey = argument('key') ?? '';
const appURL = (argument('app') ?? '').replace(/\/$/, '');
const months = (argument('months') ?? '').split(',').filter(Boolean);
const apply = process.argv.includes('--apply');
if (!deviceURL || !projectURL || !serviceRoleKey || !appURL || months.length === 0) {
	throw new Error('pass --device, --url, --key, --app and --months');
}

type DeviceEvent = {
	kind?: string;
	occurredAt?: string;
	mattermostUserID?: string;
	locationName?: string;
	canceledAt?: string;
};

const admin = createClient(projectURL, serviceRoleKey, {
	auth: { autoRefreshToken: false, persistSession: false }
});

async function deviceEvents(month: string): Promise<DeviceEvent[]> {
	const response = await fetch(`${deviceURL}/attendance/api/summary?month=${month}`);
	if (!response.ok) throw new Error(`the device answered ${response.status} for ${month}`);
	const answered = (await response.json()) as { events?: DeviceEvent[] };
	return answered.events ?? [];
}

const companies = await admin.from('company').select('id, name').returns<{ id: string; name: string }[]>();
if (companies.error) throw new Error(companies.error.message);
if ((companies.data ?? []).length !== 1) {
	throw new Error(`this script reconciles one company; the record holds ${companies.data?.length}`);
}
const company = companies.data[0];
console.log(`company: ${company.name}`);

const agent = await issueAgentKey(admin, company.id, `reconcile-${crypto.randomUUID().slice(0, 8)}`);
try {
	for (const month of months) {
		const { from, to } = windowOf(month);
		const events = (await deviceEvents(month))
			.filter((event) => event.mattermostUserID && event.kind && event.occurredAt && !event.canceledAt)
			.filter((event) => (event.occurredAt ?? '') >= from && (event.occurredAt ?? '') < to)
			.map((event) => ({
				externalID: event.mattermostUserID,
				kind: event.kind,
				occurredAt: event.occurredAt,
				location: event.locationName ?? ''
			}));

		if (!apply) {
			console.log(`${month}  ${events.length} events the device calls true (dry run)`);
			continue;
		}

		const response = await fetch(`${appURL}/api/agent/attendance-reconcile`, {
			method: 'POST',
			headers: { Authorization: `Bearer ${agent.apiKey}`, 'Content-Type': 'application/json' },
			body: JSON.stringify({ platform: 'mattermost', from, to, events })
		});
		const answered = await response.text();
		console.log(`${month}  ${response.status}  ${answered.slice(0, 200)}`);
	}
} finally {
	await revokeAgent(admin, agent.agentID);
	console.log('the key this script issued is revoked');
}
