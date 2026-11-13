// A Workers route names a hostname, and a hostname spelled in wrangler.jsonc is
// the fleet domain written down a second time. It was `updates.example.test`
// there while the live route answered `updates.intern.kim`, so the route that
// serves releases had nothing in the tree that explained it.

import { defaultZone } from '../src/lib/server/fleet-domain';

export function zoneOfSettings(settings: Record<string, string | undefined>): string {
	const configured = settings.CLOUDFLARE_DOMAIN ?? '';
	return configured.trim().toLowerCase() || defaultZone;
}

export function routePatternOfSubdomain(label: string, zone: string): string {
	const subdomain = label.trim().toLowerCase();
	if (!subdomain) throw new Error('name the subdomain the worker answers on');
	if (subdomain.includes('.')) throw new Error(`${label} is a hostname, not a subdomain; the zone comes from companyzone`);
	return `${subdomain}.${zone}/*`;
}

export type LiveRoute = { pattern: string; script: string };

export function liveRoutesIn(listed: unknown): LiveRoute[] {
	if (!Array.isArray(listed)) throw new Error('Cloudflare listed the zone routes as something other than a list');
	return listed.flatMap((route) =>
		typeof route?.pattern === 'string' && typeof route?.script === 'string'
			? [{ pattern: route.pattern, script: route.script }]
			: []
	);
}

export function routesToDeploy(liveRoutes: LiveRoute[], workerName: string, namedPatterns: string[]): string[] {
	const heldByTheWorker = liveRoutes.filter((route) => route.script === workerName).map((route) => route.pattern);
	return [...new Set([...heldByTheWorker, ...namedPatterns])];
}
