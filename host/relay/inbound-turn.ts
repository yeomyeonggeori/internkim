import type { Addressing, AskedPermission, BlueclawACPClient } from './acp-session';
import { readInboundMessage, type InboundMessage } from './inbound-message';
import type { InboundQueue, QueuedInboundEvent } from './inbound-queue';

export type InboundTurnSettings = {
	client: BlueclawACPClient;
	queue: InboundQueue;
	postToConversation: (addressing: Addressing, message: string) => Promise<void>;
	waitBeforeRetrying?: (milliseconds: number) => Promise<void>;
	report?: (line: string) => void;
};

type PendingQuestion = {
	answer: (words: string) => void;
};

type RunningTurn = {
	eventKey: string;
	/** Set once the agent asks something, which only a started run can do. */
	blueclawOpenedARun: boolean;
};

const firstRetryDelayMilliseconds = 250;
const longestRetryDelayMilliseconds = 30_000;

export class InboundTurns {
	private readonly settings: InboundTurnSettings;
	private readonly pendingByConversation = new Map<string, PendingQuestion>();
	/** The event each running turn is still owed, by conversation. */
	private readonly turnInFlight = new Map<string, RunningTurn>();
	private draining: Promise<void> = Promise.resolve();

	constructor(settings: InboundTurnSettings) {
		this.settings = settings;
	}

	askThePerson = async (asked: AskedPermission, addressing: Addressing): Promise<string> => {
		const running = this.turnInFlight.get(addressing.conversationID);
		if (running) running.blueclawOpenedARun = true;
		const answering = new Promise<string>((resolve) => {
			this.pendingByConversation.set(addressing.conversationID, { answer: resolve });
		});
		await this.settings.postToConversation(addressing, asked.question);
		return answering;
	};

	/**
	 * Answers only once the event is on disk, so the 202 chatd retries until it
	 * gets is a promise the message will be delivered, not that it was heard.
	 */
	async keep(key: string, body: unknown): Promise<boolean> {
		const isNew = await this.settings.queue.keep(key, body);
		this.startDraining();
		return isNew;
	}

	startDraining(): void {
		this.draining = this.draining.then(() => this.drainOnce()).catch((failure) => {
			this.settings.report?.(`the inbound queue would not drain: ${String(failure)}`);
		});
	}

	async settled(): Promise<void> {
		await this.draining;
	}

	private async drainOnce(): Promise<void> {
		for (const event of await this.settings.queue.undelivered()) {
			const inbound = readInboundMessage(event.body);
			if (!inbound) {
				this.settings.report?.(`dropped ${event.key}: it is not a message the agent can answer`);
				await this.settings.queue.forget(event.key);
				continue;
			}
			const conversationID = inbound.addressing.conversationID;
			// The event a running turn was started on is still queued until that
			// turn finishes, and it is not the answer to the question it asked.
			const running = this.turnInFlight.get(conversationID);
			const isTheEventItsOwnTurnIsRunningOn = running?.eventKey === event.key;
			if (isTheEventItsOwnTurnIsRunningOn) continue;
			const pending = this.pendingByConversation.get(conversationID);
			if (pending) {
				this.pendingByConversation.delete(conversationID);
				pending.answer(inbound.message);
				await this.settings.queue.forget(event.key);
				continue;
			}
			if (running) continue;
			this.beginTurn(event, inbound);
		}
	}

	/**
	 * The turn is not awaited here: a turn that stopped to ask something is
	 * waiting for a message that has to come through this same queue.
	 */
	private beginTurn(event: QueuedInboundEvent, inbound: InboundMessage): void {
		const conversationID = inbound.addressing.conversationID;
		const running: RunningTurn = { eventKey: event.key, blueclawOpenedARun: false };
		this.turnInFlight.set(conversationID, running);
		void this.runTurn(event, inbound, running).finally(() => {
			this.turnInFlight.delete(conversationID);
			this.startDraining();
		});
	}

	private async runTurn(
		event: QueuedInboundEvent,
		inbound: InboundMessage,
		running: RunningTurn
	): Promise<void> {
		try {
			const answered = await this.settings.client.ask(
				inbound.requester,
				inbound.addressing,
				inbound.message,
				{
					messageID: inbound.messageID,
					replyTargetID: inbound.addressing.replyTargetID,
					isThread: inbound.addressing.isThread,
					context: inbound.context
				}
			);
			if (answered.reply) {
				await this.settings.postToConversation(inbound.addressing, answered.reply);
			}
			for (const line of answered.progress) {
				this.settings.report?.(`progress: ${line}`);
			}
			await this.settings.queue.forget(event.key);
		} catch (failure) {
			if (running.blueclawOpenedARun) {
				await this.leaveItToBlueclaw(event, failure);
				return;
			}
			await this.giveItAnotherGo(event, failure);
		}
	}

	private async leaveItToBlueclaw(event: QueuedInboundEvent, failure: unknown): Promise<void> {
		this.settings.report?.(`${event.key} is blueclaw's to finish: ${String(failure)}`);
		await this.settings.queue.forget(event.key);
	}

	private async giveItAnotherGo(event: QueuedInboundEvent, failure: unknown): Promise<void> {
		const attempts = await this.settings.queue.recordAttempt(event.key);
		const attempted = { ...event, attempts };
		if (this.settings.queue.hasExhausted(attempted)) {
			this.settings.report?.(
				`dropped ${event.key} after ${attempts} attempts: ${String(failure)}`
			);
			await this.settings.queue.forget(event.key);
			return;
		}
		this.settings.report?.(`${event.key} failed on attempt ${attempts}: ${String(failure)}`);
		await this.waitBeforeRetrying(attempts);
	}

	private waitBeforeRetrying(attempts: number): Promise<void> {
		const delay = Math.min(
			firstRetryDelayMilliseconds * 2 ** (attempts - 1),
			longestRetryDelayMilliseconds
		);
		const wait = this.settings.waitBeforeRetrying ?? ((milliseconds) => Bun.sleep(milliseconds));
		return wait(delay);
	}
}
