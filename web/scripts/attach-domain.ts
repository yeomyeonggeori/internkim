// Points a hostname at the Pages project. A wildcard is what makes a company's
// address exist the moment the company does — nothing is created per company.
//   bun run web/scripts/attach-domain.ts --domain '*.example.test' [--list]

const token = process.env.CF_API_TOKEN ?? process.env.CLOUDFLARE_API_TOKEN ?? '';
if (!token) throw new Error('set CF_API_TOKEN');

function argument(name: string): string | undefined {
	const index = process.argv.indexOf(`--${name}`);
	return index >= 0 ? process.argv[index + 1] : undefined;
}

const accountID = argument('account') ?? '694280310d0ed1189a2a54c4a546403e';
const project = argument('project') ?? 'internkim-app';
const domain = argument('domain');
const base = `https://api.cloudflare.com/client/v4/accounts/${accountID}/pages/projects/${project}/domains`;
const headers = { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' };

if (process.argv.includes('--list') || !domain) {
	const response = await fetch(base, { headers });
	const body = await response.json();
	if (!body.success) throw new Error(JSON.stringify(body.errors));
	for (const entry of body.result ?? []) {
		console.log(`  ${entry.name.padEnd(24)} ${entry.status}`);
	}
	if ((body.result ?? []).length === 0) console.log('  no domains attached');
	process.exit(0);
}

const response = await fetch(base, { method: 'POST', headers, body: JSON.stringify({ name: domain }) });
const body = await response.json();
if (!body.success) throw new Error(JSON.stringify(body.errors));
console.log(`attached ${body.result.name} — ${body.result.status}`);
