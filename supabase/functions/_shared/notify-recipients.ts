export type AskedRecipients = {
	platform?: unknown;
	externalIDs?: unknown;
	emails?: unknown;
};

export type Addressed =
	| { by: 'email'; keys: string[] }
	| { by: 'messenger'; platform: string; keys: string[] };

export function addressedIn(asked: AskedRecipients): Addressed | null {
	const emails = strings(asked.emails).map((address) => address.toLowerCase());
	if (emails.length > 0) return { by: 'email', keys: emails };

	const platform = typeof asked.platform === 'string' ? asked.platform.trim() : '';
	if (!platform || !Array.isArray(asked.externalIDs)) return null;
	return { by: 'messenger', platform, keys: strings(asked.externalIDs) };
}

function strings(offered: unknown): string[] {
	if (!Array.isArray(offered)) return [];
	return offered
		.filter((entry): entry is string => typeof entry === 'string')
		.map((entry) => entry.trim())
		.filter((entry) => entry !== '');
}
