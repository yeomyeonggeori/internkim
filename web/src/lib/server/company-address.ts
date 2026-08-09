export type CompanyAddress = {
	hostname: string | null;
	status: string;
};

type CloudflareSettings = {
	token: string;
	accountID: string;
	zoneID: string;
	project: string;
	zone: string;
};

export async function claimCompanyAddress(
	environment: Record<string, string | undefined>,
	slug: string,
	servedFrom: string,
): Promise<CompanyAddress> {
	const cloudflare = cloudflareSettings(environment, servedFrom);
	if (!cloudflare) return { hostname: null, status: 'addresses are not configured' };

	const hostname = `${slug}.${cloudflare.zone}`;
	const pointed = await pointHostnameAtProject(cloudflare, hostname);
	if (pointed) return { hostname, status: pointed };

	return { hostname, status: await attachHostnameToProject(cloudflare, hostname) };
}

function cloudflareSettings(
	environment: Record<string, string | undefined>,
	servedFrom: string,
): CloudflareSettings | null {
	const settings = {
		token: environment.CF_API_TOKEN ?? '',
		accountID: environment.CF_ACCOUNT_ID ?? '',
		zoneID: environment.CF_ZONE_ID ?? '',
		project: environment.CF_PAGES_PROJECT ?? '',
		zone: environment.CF_DOMAIN || zoneOf(servedFrom),
	};
	return Object.values(settings).every(Boolean) ? settings : null;
}

async function pointHostnameAtProject(cloudflare: CloudflareSettings, hostname: string): Promise<string> {
	const target = await projectSubdomain(cloudflare);
	if (!target) return 'the Pages project has no address of its own to point at';

	const answer = await callCloudflare(cloudflare, `zones/${cloudflare.zoneID}/dns_records`, {
		type: 'CNAME',
		name: hostname,
		content: target,
		proxied: true,
	});
	if (answer.success) return '';
	return alreadyExists(answer.errors) ? '' : `no record: ${JSON.stringify(answer.errors)}`;
}

async function attachHostnameToProject(cloudflare: CloudflareSettings, hostname: string): Promise<string> {
	const answer = await callCloudflare(
		cloudflare,
		`accounts/${cloudflare.accountID}/pages/projects/${cloudflare.project}/domains`,
		{ name: hostname },
	);
	if (!answer.success) return alreadyExists(answer.errors) ? 'claimed' : `not claimed: ${JSON.stringify(answer.errors)}`;
	return (answer.result as { status?: string } | undefined)?.status ?? 'claimed';
}

async function projectSubdomain(cloudflare: CloudflareSettings): Promise<string> {
	const answer = await callCloudflare(
		cloudflare,
		`accounts/${cloudflare.accountID}/pages/projects/${cloudflare.project}`,
	);
	return (answer.result as { subdomain?: string } | undefined)?.subdomain ?? '';
}

type CloudflareAnswer = { success?: boolean; result?: unknown; errors?: { code?: number }[] };

async function callCloudflare(
	cloudflare: CloudflareSettings,
	path: string,
	body?: unknown,
): Promise<CloudflareAnswer> {
	const response = await fetch(`https://api.cloudflare.com/client/v4/${path}`, {
		method: body ? 'POST' : 'GET',
		headers: { Authorization: `Bearer ${cloudflare.token}`, 'Content-Type': 'application/json' },
		body: body ? JSON.stringify(body) : undefined,
	});
	return (await response.json()) as CloudflareAnswer;
}

function alreadyExists(errors: { code?: number }[] | undefined): boolean {
	return (errors ?? []).some((entry) => entry.code === 81053 || entry.code === 81057 || entry.code === 8000018);
}

function zoneOf(servedFrom: string): string {
	const parts = servedFrom.split('.');
	return parts.length >= 2 ? parts.slice(-2).join('.') : '';
}
