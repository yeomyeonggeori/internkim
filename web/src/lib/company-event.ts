export type CompanyEvent = {
	kind: string;
	conversationID?: string;
	messageID?: string;
};

export function companyEventOf(offered: unknown): CompanyEvent | null {
	if (typeof offered !== 'object' || offered === null) return null;
	const { kind, conversationID, messageID } = offered as Record<string, unknown>;
	if (typeof kind !== 'string' || kind === '') return null;
	return {
		kind,
		...(typeof conversationID === 'string' ? { conversationID } : {}),
		...(typeof messageID === 'string' ? { messageID } : {})
	};
}
