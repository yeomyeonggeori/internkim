import { RequestError } from '@agentclientprotocol/sdk';
import {
	AgentUnreachable,
	type Addressing,
	type AskedPermission,
	type BlueclawACPClient
} from './acp-session';
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
	finished: Promise<void>;
};

const firstRetryDelayMilliseconds = 250;
const longestRetryDelayMilliseconds = 30_000;

export class InboundTurns {
	private readonly settings: InboundTurnSettings;
	private readonly pendingByConversation = new Map<string, PendingQuestion>();
	/** The event each running turn is still owed, by conversation. */
	private readonly turnInFlight = new Map<string, RunningTurn>();
	/** How many times in a row the agent could not be reached for each event; no ceiling ends this. */
	private readonly unreachedInARow = new Map<string, number>();
	private draining: Promise<void> = Promise.resolve();

	constructor(settings: InboundTurnSettings) {
		this.settings = settings;
	}

	/** Puts every question this relay had not delivered before it last stopped back in play. */
	async restoreHeldQuestions(): Promise<void> {
		await this.settings.client.restoreOutstandingQuestions();
	}

	askThePerson = async (asked: AskedPermission, addressing: Addressing): Promise<string> => {
		const running = this.turnInFlight.get(addressing.conversationID);
		if (running) await this.handTheRunToBlueclaw(running);
		const answering = this.awaitAnAlreadyAskedQuestion(addressing);
		await this.settings.postToConversation(addressing, asked.question);
		return answering;
	};

	private async handTheRunToBlueclaw(running: RunningTurn): Promise<void> {
		running.blueclawOpenedARun = true;
		await this.forget(running.eventKey);
	}

	/** The question already reached the person before this relay last stopped; only wait. */
	awaitAnAlreadyAskedQuestion = (addressing: Addressing): Promise<string> => {
		return new Promise<string>((resolve) => {
			this.pendingByConversation.set(addressing.conversationID, { answer: resolve });
		});
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

	/**
	 * Resolves once nothing is left to happen on its own: the queue has been
	 * drained and every turn not parked on a question has finished. A parked
	 * turn is waiting for a message that has to come through this same queue.
	 */
	async settled(): Promise<void> {
		await this.draining;
		const finishing = [...this.turnInFlight]
			.filter(([conversationID]) => !this.pendingByConversation.has(conversationID))
			.map(([, running]) => running.finished);
		if (finishing.length === 0) return;
		await Promise.all(finishing);
		await this.settled();
	}

	private async drainOnce(): Promise<void> {
		for (const event of await this.settings.queue.undelivered()) {
			const inbound = readInboundMessage(event.body);
			if (!inbound) {
				this.settings.report?.(`dropped ${event.key}: it is not a message the agent can answer`);
				await this.forget(event.key);
				continue;
			}
			const conversationID = inbound.addressing.conversationID;
			// The event a running turn was started on stays queued until the turn
			// finishes or asks something, and it is not an answer to anything.
			const running = this.turnInFlight.get(conversationID);
			const isTheEventItsOwnTurnIsRunningOn = running?.eventKey === event.key;
			if (isTheEventItsOwnTurnIsRunningOn) continue;
			const pending = this.pendingByConversation.get(conversationID);
			if (pending) {
				this.pendingByConversation.delete(conversationID);
				pending.answer(inbound.message);
				await this.forget(event.key);
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
		const running: RunningTurn = { eventKey: event.key, blueclawOpenedARun: false, finished: Promise.resolve() };
		this.turnInFlight.set(conversationID, running);
		running.finished = this.runTurn(event, inbound, running).finally(() => {
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
			await this.forget(event.key);
		} catch (failure) {
			if (running.blueclawOpenedARun) {
				await this.leaveItToBlueclaw(event, failure);
				return;
			}
			if (failure instanceof AgentUnreachable) {
				await this.waitForTheAgent(event, failure);
				return;
			}
			this.unreachedInARow.delete(event.key);
			await this.giveItAnotherGo(event, failure);
		}
	}

	private async forget(key: string): Promise<void> {
		this.unreachedInARow.delete(key);
		await this.settings.queue.forget(key);
	}

	private async waitForTheAgent(event: QueuedInboundEvent, failure: AgentUnreachable): Promise<void> {
		const unreached = (this.unreachedInARow.get(event.key) ?? 0) + 1;
		this.unreachedInARow.set(event.key, unreached);
		this.settings.report?.(
			`${event.key} stays queued until the agent takes it (${unreached} in a row): ${failure.message}`
		);
		await this.waitBeforeRetrying(unreached);
	}

	private async leaveItToBlueclaw(event: QueuedInboundEvent, failure: unknown): Promise<void> {
		this.settings.report?.(`${event.key} is blueclaw's to finish: ${described(failure)}`);
		await this.forget(event.key);
	}

	private async giveItAnotherGo(event: QueuedInboundEvent, failure: unknown): Promise<void> {
		const attempts = await this.settings.queue.recordAttempt(event.key);
		const attempted = { ...event, attempts };
		if (this.settings.queue.hasExhausted(attempted)) {
			this.settings.report?.(
				`dropped ${event.key} after ${attempts} attempts: ${described(failure)}`
			);
			await this.forget(event.key);
			return;
		}
		this.settings.report?.(`${event.key} failed on attempt ${attempts}: ${described(failure)}`);
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

/** A refusal the agent answered with carries its reason in the data, not in the message. */
function described(failure: unknown): string {
	if (failure instanceof RequestError && failure.data !== undefined) {
		return `${String(failure)} ${JSON.stringify(failure.data)}`;
	}
	return String(failure);
}
