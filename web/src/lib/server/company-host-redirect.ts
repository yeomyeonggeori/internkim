import { defaultZone } from './fleet-domain';
import { protectedResourceMetadataPath } from './public-api/protected-resource';

export type CompanyHostQuestion = {
	hostname: string;
	zone: string;
	pathname: string;
};

function bareHost(value: string): string {
	return value.trim().toLowerCase();
}

export function theAppAddressOf(environment: { CLOUDFLARE_DOMAIN?: string }): string {
	return `https://${bareHost(environment.CLOUDFLARE_DOMAIN || defaultZone)}`;
}

// fetch drops Authorization across origins, and every attached hostname is the
// same deployment, so these are answered where they land.
function carriesACallerCredential(pathname: string): boolean {
	return pathname.startsWith('/api/') || pathname.startsWith('/v1/');
}

// RFC 9728 §3.3: a client rejects metadata whose resource is not the URL it asked about.
function describesTheHostItWasAskedOn(pathname: string): boolean {
	return pathname.startsWith(`${protectedResourceMetadataPath}/`);
}

export function movesToTheOneAddress(question: CompanyHostQuestion): boolean {
	const hostname = bareHost(question.hostname);
	const zone = bareHost(question.zone);
	if (!hostname || !zone) return false;
	if (carriesACallerCredential(question.pathname)) return false;
	if (describesTheHostItWasAskedOn(question.pathname)) return false;
	if (hostname === zone) return false;
	return hostname.endsWith(`.${zone}`);
}
