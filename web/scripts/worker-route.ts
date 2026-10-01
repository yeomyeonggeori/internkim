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
	if (subdomain.includes('.')) throw new Error(`${label} is a hostname, not a subdomain; the zone comes from fleetdomain`);
	return `${subdomain}.${zone}/*`;
}
