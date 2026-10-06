import { RequestError } from '@agentclientprotocol/sdk';
import { AgentUnreachable, type BlueclawACPClient } from './acp-session';
import { readInboundMessage, type InboundMessage } from './inbound-message';
import type { InboundQueue, QueuedInboundEvent } from './inbound-queue';

export type InboundTurnSettings = {
	client: BlueclawACPClient;
	queue: InboundQueue;
	waitBeforeRetrying?: (milliseconds: number) => Promise<void>;
	report?: (line: string) => void;
};

type RunningTurn = {
	eventKey: string;
	conversationID: string;
	messageID: string;
	isHandedToBlueclaw: boolean;
	finished: Promise<void>;
};

const firstRetryDelayMilliseconds = 250;
const longestRetryDelayMilliseconds = 30_000;

export class InboundTurns {
	private readonly settings: InboundTurnSettings;
	private readonly turnsInFlight = new Map<string, RunningTurn>();
	private readonly unreachedInARow = new Map<string, number>();
	private draining: Promise<void> = Promise.resolve();

	constructor(settings: InboundTurnSettings) {
		this.settings = settings;
	}

	releaseTurnsWaitingOnApproval = (conversationID: string): void => {
		const waiting = [...this.turnsInFlight.values()].filter(
			(running) => running.conversationID === conversationID && this.isWaiting(running)
		);
		for (const running of waiting) running.isHandedToBlueclaw = true;
		Promise.all(waiting.map((running) => this.forget(running.eventKey)))
			.catch((failure) => this.settings.report?.(`a waiting turn would not leave the queue: ${String(failure)}`))
			.finally(() => this.startDraining());
	};

	private isWaiting(running: RunningTurn): boolean {
		return this.settings.client.isWaitingOnApproval(running.conversationID, running.messageID);
	}

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
		const finishing = this.activeTurns().map((running) => running.finished);
		if (finishing.length === 0) return;
		await Promise.all(finishing);
		await this.settled();
	}

	private async drainOnce(): Promise<void> {
		for (const event of await this.settings.queue.undelivered()) {
			const inbound = readInboundMessage(event.body);
			if (!inbound) {
				await this.forget(event.key);
				this.settings.report?.(`dropped ${event.key}: it is not a message the agent can answer`);
				continue;
			}
			if (this.turnsInFlight.has(event.key)) continue;
			if (await this.isConsumedAsApprovalAnswer(event, inbound)) continue;
			if (this.activeTurnIn(inbound.addressing.conversationID)) continue;
			this.beginTurn(event, inbound);
		}
	}

	private async isConsumedAsApprovalAnswer(event: QueuedInboundEvent, inbound: InboundMessage): Promise<boolean> {
		if (event.isPrompt) return false;
		if (!this.settings.client.hasPendingApprovalIn(inbound.addressing.conversationID)) return false;
		const { addressing, messageID, message } = inbound;
		const isAnswer = await this.settings.client.answerPendingApproval(addressing, messageID, message);
		if (!isAnswer) {
			await this.settings.queue.recordAsPrompt(event.key);
			return false;
		}
		await this.forget(event.key);
		return true;
	}

	private activeTurnIn(conversationID: string): RunningTurn | undefined {
		return this.activeTurns().find((running) => running.conversationID === conversationID);
	}

	private activeTurns(): RunningTurn[] {
		return [...this.turnsInFlight.values()].filter((running) => !this.isWaiting(running));
	}

	private beginTurn(event: QueuedInboundEvent, inbound: InboundMessage): void {
		const running: RunningTurn = {
			eventKey: event.key,
			conversationID: inbound.addressing.conversationID,
			messageID: inbound.messageID,
			isHandedToBlueclaw: false,
			finished: Promise.resolve()
		};
		this.turnsInFlight.set(event.key, running);
		running.finished = this.runTurn(event, inbound, running).finally(() => {
			this.turnsInFlight.delete(event.key);
			this.startDraining();
		});
	}

	private async runTurn(
		event: QueuedInboundEvent,
		inbound: InboundMessage,
		running: RunningTurn
	): Promise<void> {
		try {
			if (await this.isAnsweredOnceTheAgentIsBack(event, inbound)) return;
			await this.settings.client.ask(inbound.requester, inbound.addressing, inbound.message, {
				messageID: inbound.messageID,
				replyTargetID: inbound.addressing.replyTargetID,
				isThread: inbound.addressing.isThread,
				context: inbound.context
			});
			await this.forget(event.key);
		} catch (failure) {
			if (running.isHandedToBlueclaw) {
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

	private async isAnsweredOnceTheAgentIsBack(event: QueuedInboundEvent, inbound: InboundMessage): Promise<boolean> {
		const wasJudgedAlready = event.isPrompt || this.settings.client.hasPendingApprovalIn(inbound.addressing.conversationID);
		await this.settings.client.connect().catch(() => undefined);
		if (wasJudgedAlready) return false;
		return this.isConsumedAsApprovalAnswer(event, inbound);
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
			await this.forget(event.key);
			this.settings.report?.(
				`dropped ${event.key} after ${attempts} attempts: ${described(failure)}`
			);
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

function described(failure: unknown): string {
	if (failure instanceof RequestError && failure.data !== undefined) {
		return `${String(failure)} ${JSON.stringify(failure.data)}`;
	}
	return String(failure);
}
