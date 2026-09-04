import type { Addressing, AskedPermission, BlueclawACPClient } from './acp-session';
import type { InboundMessage } from './inbound-message';

export type TurnAnswer = {
	reply: string;
	stopReason: string;
};

export type InboundTurnSettings = {
	client: BlueclawACPClient;
	postToConversation: (addressing: Addressing, message: string) => Promise<void>;
	report?: (line: string) => void;
};

type PendingQuestion = {
	answer: (words: string) => void;
};

export class InboundTurns {
	private readonly settings: InboundTurnSettings;
	private readonly pendingByConversation = new Map<string, PendingQuestion>();

	constructor(settings: InboundTurnSettings) {
		this.settings = settings;
	}

	askThePerson = async (asked: AskedPermission, addressing: Addressing): Promise<string> => {
		const answering = new Promise<string>((resolve) => {
			this.pendingByConversation.set(addressing.conversationID, { answer: resolve });
		});
		await this.settings.postToConversation(addressing, asked.question);
		return answering;
	};

	async receive(inbound: InboundMessage): Promise<TurnAnswer | null> {
		const pending = this.pendingByConversation.get(inbound.addressing.conversationID);
		if (pending) {
			this.pendingByConversation.delete(inbound.addressing.conversationID);
			pending.answer(inbound.message);
			return null;
		}
		const answered = await this.settings.client.ask(
			inbound.requester,
			inbound.addressing,
			inbound.message
		);
		if (answered.reply) {
			await this.settings.postToConversation(inbound.addressing, answered.reply);
		}
		for (const line of answered.progress) {
			this.settings.report?.(`progress: ${line}`);
		}
		return { reply: answered.reply, stopReason: answered.stopReason };
	}
}
