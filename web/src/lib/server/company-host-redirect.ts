export type CompanyHostQuestion = {
	hostname: string;
	zone: string;
	apiHostname: string;
};

export function movesToTheOneAddress(question: CompanyHostQuestion): boolean {
	const hostname = question.hostname.trim().toLowerCase();
	const zone = question.zone.trim().toLowerCase();
	if (!hostname || !zone) return false;
	if (hostname === zone) return false;
	if (hostname === question.apiHostname.trim().toLowerCase()) return false;
	return hostname.endsWith(`.${zone}`);
}
