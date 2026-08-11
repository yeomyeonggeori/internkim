export type CompanyHostQuestion = {
	hostname: string;
	zone: string;
};

function bareHost(value: string): string {
	return value.trim().toLowerCase();
}

export function appHostnameOf(zone: string): string {
	return `app.${bareHost(zone)}`;
}

export function apiHostnameOf(zone: string): string {
	return `api.${bareHost(zone)}`;
}

export function movesToTheOneAddress(question: CompanyHostQuestion): boolean {
	const hostname = bareHost(question.hostname);
	const zone = bareHost(question.zone);
	if (!hostname || !zone) return false;
	if (hostname === zone) return false;
	if (hostname === appHostnameOf(zone) || hostname === apiHostnameOf(zone)) return false;
	return hostname.endsWith(`.${zone}`);
}
