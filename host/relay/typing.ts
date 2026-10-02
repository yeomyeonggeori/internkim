import { externalIDs } from './arrived';
import { typingEventKind } from '../../web/src/lib/messenger/typing-signal';

export const typingPath = '/typing';
const memberMemoryMilliseconds = 5 * 60_000;

export type Typing = {
	conversationID: string;
	authorExternalID: string;
	recipientExternalIDs: string[];
};

export type TypingTellerDependencies = {
	memberIDsOf: (externalIDs: string[]) => Promise<Map<string, string>>;
	deliver: (event: Record<string, unknown>, audienceMemberIDs: string[]) => void;
	now: () => number;
};

export function readTyping(offered: unknown): Typing | null {
	if (typeof offered !== 'object' || offered === null) return null;
	const held = offered as Record<string, unknown>;
	const conversationID = text(held.conversationID);
	const authorExternalID = text(held.authorExternalID);
	if (!conversationID || !authorExternalID) return null;
	return {
		conversationID,
		authorExternalID,
		recipientExternalIDs: externalIDs(held.recipientExternalIDs, authorExternalID)
	};
}

export function typingTeller(dependencies: TypingTellerDependencies): (typing: Typing) => Promise<number> {
	const remembered = new Map<string, { memberID: string | null; readAt: number }>();

	async function memberIDsOf(externalIDs: string[]): Promise<string[]> {
		const oldestKept = dependencies.now() - memberMemoryMilliseconds;
		const unknown = externalIDs.filter((externalID) => (remembered.get(externalID)?.readAt ?? 0) <= oldestKept);
		if (unknown.length > 0) {
			const found = await dependencies.memberIDsOf(unknown);
			for (const externalID of unknown) {
				remembered.set(externalID, { memberID: found.get(externalID) ?? null, readAt: dependencies.now() });
			}
		}
		return externalIDs
			.map((externalID) => remembered.get(externalID)?.memberID ?? null)
			.filter((memberID): memberID is string => memberID !== null);
	}

	return async (typing) => {
		const audience = await memberIDsOf(typing.recipientExternalIDs);
		if (audience.length === 0) return 0;
		dependencies.deliver(
			{
				kind: typingEventKind,
				conversationID: typing.conversationID,
				authorExternalID: typing.authorExternalID
			},
			audience
		);
		return audience.length;
	};
}

function text(offered: unknown): string {
	return typeof offered === 'string' ? offered.trim() : '';
}
