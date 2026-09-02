import { defaultZone, docsHost } from './fleet-domain';

export function apiReferenceHomeFor(pathname: string, zone: string = defaultZone): string | null {
	if (pathname === '/api-docs' || pathname.startsWith('/api-docs/')) return `https://${docsHost(zone)}/api`;
	const document = pathname.match(/^\/openapi\/(en|ko)\.json$/);
	if (document) return `https://${docsHost(zone)}/openapi/${document[1]}.json`;
	return null;
}
