export type CompanyEvent = {
	kind: string;
	conversationID?: string;
	messageID?: string;
	authorExternalID?: string;
	transferID?: string;
	copiedBytes?: number;
	totalBytes?: number;
	status?: number;
	error?: string;
	result?: unknown;
};

export function companyEventOf(offered: unknown): CompanyEvent | null {
	if (typeof offered !== 'object' || offered === null) return null;
	const { kind, conversationID, messageID, authorExternalID, transferID, copiedBytes, totalBytes, status, error, result } =
		offered as Record<string, unknown>;
	if (typeof kind !== 'string' || kind === '') return null;
	return {
		kind,
		...(typeof conversationID === 'string' ? { conversationID } : {}),
		...(typeof messageID === 'string' ? { messageID } : {}),
		...(typeof authorExternalID === 'string' ? { authorExternalID } : {}),
		...(typeof transferID === 'string' ? { transferID } : {}),
		...(typeof copiedBytes === 'number' ? { copiedBytes } : {}),
		...(typeof totalBytes === 'number' ? { totalBytes } : {}),
		...(typeof status === 'number' ? { status } : {}),
		...(typeof error === 'string' ? { error } : {}),
		...(result !== undefined ? { result } : {})
	};
}
