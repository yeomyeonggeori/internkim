import { externalIDs } from './arrived';

export const withdrawalsPath = '/withdrawn';
const rememberedPerConversation = 20;

export type Withdrawal = {
	conversationID: string;
	messageID: string;
	recipientExternalIDs: string[];
};

export type WithdrawRequest = {
	platform: string;
	externalIDs: string[];
	conversationID: string;
	messageID: string;
};

export function readWithdrawal(offered: unknown): Withdrawal | null {
	if (typeof offered !== 'object' || offered === null) return null;
	const held = offered as Record<string, unknown>;
	const conversationID = text(held.conversationID);
	const messageID = text(held.messageID);
	const authorExternalID = text(held.authorExternalID);
	if (!conversationID || !messageID || !authorExternalID) return null;
	return {
		conversationID,
		messageID,
		recipientExternalIDs: externalIDs(held.recipientExternalIDs, authorExternalID)
	};
}

export function withdrawRequestOf(withdrawal: Withdrawal, platform: string): WithdrawRequest {
	return {
		platform,
		externalIDs: withdrawal.recipientExternalIDs,
		conversationID: withdrawal.conversationID,
		messageID: withdrawal.messageID
	};
}

export type WithdrawalTellerDependencies = {
	withdrawn: WithdrawnMessages;
	platform: string;
	askTheProject: (request: WithdrawRequest) => Promise<{ reached?: number }>;
};

export function withdrawalTeller(dependencies: WithdrawalTellerDependencies): (withdrawal: Withdrawal) => Promise<number> {
	return async (withdrawal) => {
		dependencies.withdrawn.remember(withdrawal);
		if (withdrawal.recipientExternalIDs.length === 0) return 0;
		const answered = await dependencies.askTheProject(withdrawRequestOf(withdrawal, dependencies.platform));
		return answered.reached ?? 0;
	};
}

export class WithdrawnMessages {
	private readonly byConversation = new Map<string, string[]>();

	remember(withdrawal: Withdrawal): void {
		const kept = this.byConversation.get(withdrawal.conversationID) ?? [];
		const withThisOne = [...kept.filter((messageID) => messageID !== withdrawal.messageID), withdrawal.messageID];
		this.byConversation.set(withdrawal.conversationID, withThisOne.slice(-rememberedPerConversation));
	}

	in(conversationID: string): string[] {
		return this.byConversation.get(conversationID) ?? [];
	}
}

function text(offered: unknown): string {
	return typeof offered === 'string' ? offered.trim() : '';
}
