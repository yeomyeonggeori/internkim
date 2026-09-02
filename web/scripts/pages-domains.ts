//   bun run web/scripts/pages-domains.ts --project <name> [--attach <hostname>] [--detach <hostname>]

import { requiredSetting } from './repository-setting';

const token = requiredSetting('CLOUDFLARE_API_TOKEN');

function argument(name: string): string | undefined {
	const index = process.argv.indexOf(`--${name}`);
	return index >= 0 ? process.argv[index + 1] : undefined;
}

const accountID = argument('account') ?? requiredSetting('CLOUDFLARE_ACCOUNT_ID');
const project = argument('project');
const attaching = argument('attach');
const detaching = argument('detach');
if (!project) throw new Error('pass --project <name>');

const base = `https://api.cloudflare.com/client/v4/accounts/${accountID}/pages/projects/${project}/domains`;
const headers = { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' };

async function callCloudflare(path: string, options: RequestInit = {}): Promise<unknown> {
	const response = await fetch(`https://api.cloudflare.com/client/v4${path}`, { ...options, headers });
	const body = (await response.json()) as { success: boolean; result?: unknown; errors?: unknown };
	if (!body.success) throw new Error(JSON.stringify(body.errors));
	return body.result;
}

async function findZoneHosting(hostname: string): Promise<{ id: string; name: string }> {
	const zones = (await callCloudflare('/zones?per_page=200')) as { id: string; name: string }[];
	const [hosting] = zones
		.filter((zone) => hostname === zone.name || hostname.endsWith(`.${zone.name}`))
		.sort((zone, otherZone) => otherZone.name.length - zone.name.length);
	if (!hosting) throw new Error(`no zone in this account hosts ${hostname}`);
	return hosting;
}

async function pointAtProject(hostname: string): Promise<void> {
	const zone = await findZoneHosting(hostname);
	const intendedRecord = { type: 'CNAME', name: hostname, content: `${project}.pages.dev`, proxied: true };
	const existing = (await callCloudflare(`/zones/${zone.id}/dns_records?name=${encodeURIComponent(hostname)}`)) as {
		id: string;
		type: string;
		content: string;
	}[];

	if (existing.length === 0) {
		await callCloudflare(`/zones/${zone.id}/dns_records`, { method: 'POST', body: JSON.stringify(intendedRecord) });
		console.log(`  created ${hostname} CNAME ${intendedRecord.content}`);
		return;
	}

	for (const record of existing) {
		if (record.type === intendedRecord.type && record.content === intendedRecord.content) {
			console.log(`  ${hostname} already names ${intendedRecord.content}`);
			continue;
		}
		await callCloudflare(`/zones/${zone.id}/dns_records/${record.id}`, { method: 'PUT', body: JSON.stringify(intendedRecord) });
		console.log(`  repointed ${hostname} from ${record.content} to ${intendedRecord.content}`);
	}
}

// A hostname belongs to one Pages project at a time, so moving it between two
// is a detach and then an attach. Detaching leaves DNS alone, which is what
// keeps a move from going dark: the record still resolves, and the attach that
// follows repoints it at the project now answering for that hostname.
if (detaching) {
	const response = await fetch(`${base}/${encodeURIComponent(detaching)}`, { method: 'DELETE', headers });
	const body = (await response.json()) as { success: boolean; errors?: unknown };
	if (!body.success) throw new Error(JSON.stringify(body.errors));
	console.log(`detached ${detaching}`);
}

// Cloudflare answers 8000018 when the hostname is already on this project. An
// attach has to stay idempotent to be worth running: the whole reason to rerun
// it is that a previous one left the DNS half of the job undone.
const alreadyAttached = 8000018;

if (attaching) {
	const response = await fetch(base, { method: 'POST', headers, body: JSON.stringify({ name: attaching }) });
	const body = (await response.json()) as {
		success: boolean;
		errors?: { code: number }[];
		result?: { name: string; status: string };
	};
	const errors = body.errors ?? [];
	const attachedBefore = errors.length > 0 && errors.every((error) => error.code === alreadyAttached);
	if (!body.success && !attachedBefore) throw new Error(JSON.stringify(errors));
	console.log(attachedBefore ? `${attaching} was already attached` : `attached ${body.result?.name} — ${body.result?.status}`);
	await pointAtProject(attaching);
}

const listed = await fetch(base, { headers });
const body = (await listed.json()) as { success: boolean; errors?: unknown; result?: { name: string; status: string }[] };
if (!body.success) throw new Error(JSON.stringify(body.errors));
for (const entry of body.result ?? []) console.log(`  ${entry.name.padEnd(28)} ${entry.status}`);
if ((body.result ?? []).length === 0) console.log('  no domains attached');
