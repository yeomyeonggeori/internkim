export type CompanyAddress = {
	hostname: string | null;
	status: string;
};

// A company's address is a hostname on the Pages project that serves this page.
// Wildcards are not accepted there, so founding a company claims its own.
export async function claimCompanyAddress(
	environment: Record<string, string | undefined>,
	slug: string,
	servedFrom: string,
): Promise<CompanyAddress> {
	const token = environment.CF_API_TOKEN ?? '';
	const accountID = environment.CF_ACCOUNT_ID ?? '';
	const project = environment.CF_PAGES_PROJECT ?? '';
	const zone = environment.COMPANY_ADDRESS_ZONE ?? zoneOf(servedFrom);
	if (!token || !accountID || !project || !zone) return { hostname: null, status: 'addresses are not configured' };

	const hostname = `${slug}.${zone}`;
	const response = await fetch(
		`https://api.cloudflare.com/client/v4/accounts/${accountID}/pages/projects/${project}/domains`,
		{
			method: 'POST',
			headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' },
			body: JSON.stringify({ name: hostname })
		}
	);
	const body = (await response.json()) as { success?: boolean; result?: { status?: string }; errors?: unknown };
	if (!body.success) return { hostname, status: `not claimed: ${JSON.stringify(body.errors)}` };
	return { hostname, status: body.result?.status ?? 'claimed' };
}

function zoneOf(servedFrom: string): string {
	const parts = servedFrom.split('.');
	return parts.length >= 2 ? parts.slice(-2).join('.') : '';
}
