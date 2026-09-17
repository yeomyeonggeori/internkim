// Cloudflare consults Workers routes before Pages, so a route left on a zone
// answers at a hostname the Pages project believes is its own, and the two
// drift apart without either side noticing.

export type CloudflareCall = (path: string, options?: RequestInit) => Promise<unknown>;

export type Domain = { name: string; status: string };
export type WorkersRoute = { id: string; pattern: string; script: string };
type Zone = { id: string; name: string };

export function hostOfRoutePattern(pattern: string): string {
	return pattern.replace(/^[a-z]+:\/\//, '').split('/')[0] ?? '';
}

export function routeCoversHostname(pattern: string, hostname: string): boolean {
	const host = hostOfRoutePattern(pattern);
	if (!host.includes('*')) return host === hostname;
	const shape = new RegExp(`^${host.split('*').map(escapeForRegExp).join('.*')}$`);
	return shape.test(hostname);
}

function escapeForRegExp(text: string): string {
	return text.replace(/[.+?^${}()|[\]\\]/g, '\\$&');
}

export async function domainsOf(call: CloudflareCall, accountID: string, project: string): Promise<Domain[]> {
	const listed = await call(`/accounts/${accountID}/pages/projects/${project}/domains`);
	return (listed as Domain[]) ?? [];
}

async function zoneHosting(call: CloudflareCall, hostname: string): Promise<Zone | null> {
	const zones = (await call('/zones?per_page=200')) as Zone[];
	const [hosting] = zones
		.filter((zone) => hostname === zone.name || hostname.endsWith(`.${zone.name}`))
		.sort((zone, otherZone) => otherZone.name.length - zone.name.length);
	return hosting ?? null;
}

export async function routesShadowing(call: CloudflareCall, hostname: string): Promise<WorkersRoute[]> {
	const zone = await zoneHosting(call, hostname);
	if (!zone) return [];
	const routes = (await call(`/zones/${zone.id}/workers/routes`)) as WorkersRoute[];
	return routes.filter((route) => routeCoversHostname(route.pattern, hostname));
}

export async function removeRoute(call: CloudflareCall, hostname: string, route: WorkersRoute): Promise<void> {
	const zone = await zoneHosting(call, hostname);
	if (!zone) throw new Error(`no zone in this account hosts ${hostname}`);
	await call(`/zones/${zone.id}/workers/routes/${route.id}`, { method: 'DELETE' });
}

export async function buildVersionAt(hostname: string): Promise<string | null> {
	const response = await fetch(`https://${hostname}/_app/version.json`, { cache: 'no-store' });
	if (!response.ok) return null;
	const body = (await response.json()) as { version?: unknown };
	return typeof body.version === 'string' ? body.version : null;
}

export function hostnamesToAnswerFor(project: string, domains: Domain[]): string[] {
	return [`${project}.pages.dev`, ...domains.filter((domain) => domain.status === 'active').map((domain) => domain.name)];
}

export async function hostnamesNotAnswering(hostnames: string[], version: string): Promise<string[]> {
	const answers = await Promise.all(hostnames.map(buildVersionAt));
	return hostnames.filter((_, index) => answers[index] !== version);
}
