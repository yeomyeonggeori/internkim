const emailedLinkFields = ['access_token', 'token_hash', 'code'];

export function carriesAnEmailedLink(hash: string, search: string): boolean {
	const inTheFragment = new URLSearchParams(hash.replace(/^#/, ''));
	const inTheQuery = new URLSearchParams(search);
	return emailedLinkFields.some((field) => inTheFragment.has(field) || inTheQuery.has(field));
}
