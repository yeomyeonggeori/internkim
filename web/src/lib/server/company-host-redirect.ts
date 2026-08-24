export type CompanyHostQuestion = {
	hostname: string;
	zone: string;
	pathname: string;
};

function bareHost(value: string): string {
	return value.trim().toLowerCase();
}

export function theOneAddressOf(zone: string): string {
	return bareHost(zone);
}

// fetch drops Authorization across origins, and every attached hostname is the
// same deployment, so these are answered where they land.
function carriesACallerCredential(pathname: string): boolean {
	return pathname.startsWith('/api/');
}

export function movesToTheOneAddress(question: CompanyHostQuestion): boolean {
	const hostname = bareHost(question.hostname);
	const zone = bareHost(question.zone);
	if (!hostname || !zone) return false;
	if (carriesACallerCredential(question.pathname)) return false;
	if (hostname === theOneAddressOf(zone)) return false;
	return hostname.endsWith(`.${zone}`);
}
