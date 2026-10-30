import type { Addressing, DeleteInConversation, EditInConversation, PostToConversation } from './acp-session';
import type { ToolStep, ToolStepStatus } from '../../web/src/lib/messenger/agent-work';

export { toolStepStatuses as toolCallStatuses } from '../../web/src/lib/messenger/agent-work';

export const startedToolCallKind = 'tool_call';
export const updatedToolCallKind = 'tool_call_update';

export type ToolCallStatus = ToolStepStatus;

export type ToolCallProgress = {
	toolCallID: string;
	title?: string;
	status?: ToolCallStatus;
};

export type StartedToolCall = {
	sessionUpdate: typeof startedToolCallKind;
	toolCallId: string;
	title: string;
	status?: ToolCallStatus | null;
};

export type UpdatedToolCall = {
	sessionUpdate: typeof updatedToolCallKind;
	toolCallId: string;
	status?: ToolCallStatus | null;
};

export function progressOfToolCall(call: StartedToolCall | UpdatedToolCall): ToolCallProgress {
	const status = call.status ?? undefined;
	if (call.sessionUpdate === startedToolCallKind) return { toolCallID: call.toolCallId, title: call.title, status };
	return { toolCallID: call.toolCallId, status };
}

export type WorkSeen = { deliveryID?: string; promptID?: string; steps: ToolStep[] };

export type WorkSaid = { deliveryID?: string; promptID?: string; messageID?: string; isStillWorking: boolean };

export type WorkWatcher = {
	working: (addressing: Addressing, work: WorkSeen) => void;
	said: (addressing: Addressing, said: WorkSaid) => void;
};

export type ToolProgressSettings = {
	postToConversation: PostToConversation;
	editInConversation: EditInConversation;
	deleteInConversation: DeleteInConversation;
	postsProgressMessages: boolean;
	work?: WorkWatcher;
	report?: (line: string) => void;
};

type DeliveryTimeline = {
	calls: Map<string, ToolStep>;
	addressing?: Addressing;
	progressMessageID?: string;
	publishing: Promise<unknown>;
};

const statusMarkers: Record<ToolCallStatus, string> = {
	pending: '○',
	in_progress: '◐',
	completed: '✓',
	failed: '✗'
};

const finishedDeliveriesRemembered = 512;

export class ToolProgress {
	private readonly settings: ToolProgressSettings;
	private readonly deliveries = new Map<string, DeliveryTimeline>();
	private readonly finishedDeliveryIDs = new Set<string>();

	constructor(settings: ToolProgressSettings) {
		this.settings = settings;
	}

	async showToolCall(deliveryID: string, addressing: Addressing, progress: ToolCallProgress): Promise<void> {
		if (this.finishedDeliveryIDs.has(deliveryID)) return;
		const timeline = this.timelineOf(deliveryID);
		const show = async () => {
			if (!this.finishedDeliveryIDs.has(deliveryID)) await this.showCall(deliveryID, timeline, addressing, progress);
		};
		await this.inOrder(timeline, show).catch((failure) => {
			this.settings.report?.(`progress was not shown: ${String(failure)}`);
		});
	}

	async showCheckpoint(deliveryID: string, addressing: Addressing, checkpoint: string): Promise<void> {
		const timeline = this.timelineOf(deliveryID);
		await this.inOrder(timeline, () => this.sayCheckpoint(deliveryID, timeline, addressing, checkpoint)).catch((failure) => {
			this.settings.report?.(`the checkpoint was not shown: ${String(failure)}`);
		});
	}

	async postFinal(deliveryID: string, addressing: Addressing, post: () => Promise<string>): Promise<string> {
		const timeline = this.timelineOf(deliveryID);
		return this.inOrder(timeline, async () => {
			const messageID = await post();
			await this.takeDownProgress(timeline, addressing);
			this.finish(deliveryID);
			this.settings.work?.said(addressing, { deliveryID, messageID, isStillWorking: false });
			return messageID;
		});
	}

	async postAlong(deliveryID: string, addressing: Addressing, post: () => Promise<string>): Promise<string> {
		const timeline = this.timelineOf(deliveryID);
		return this.inOrder(timeline, async () => {
			const messageID = await post();
			if (!this.settings.postsProgressMessages) beginNextProgress(timeline);
			this.settings.work?.said(addressing, { deliveryID, messageID, isStillWorking: true });
			return messageID;
		});
	}

	async waitForThePerson(deliveryID: string, addressing: Addressing, questionMessageID?: string): Promise<void> {
		const timeline = this.timelineOf(deliveryID);
		await this.inOrder(timeline, async () => {
			if (!this.settings.postsProgressMessages) beginNextProgress(timeline);
			this.settings.work?.said(addressing, { deliveryID, messageID: questionMessageID, isStillWorking: false });
		});
	}

	async stop(deliveryID: string): Promise<void> {
		const timeline = this.deliveries.get(deliveryID);
		if (!timeline) return;
		await this.inOrder(timeline, async () => {
			if (this.finishedDeliveryIDs.has(deliveryID)) return;
			this.finish(deliveryID);
			if (timeline.addressing) this.settings.work?.said(timeline.addressing, { deliveryID, isStillWorking: false });
		});
	}

	private timelineOf(deliveryID: string): DeliveryTimeline {
		const known = this.deliveries.get(deliveryID);
		if (known) return known;
		const timeline: DeliveryTimeline = { calls: new Map(), publishing: Promise.resolve() };
		this.deliveries.set(deliveryID, timeline);
		return timeline;
	}

	private inOrder<T>(timeline: DeliveryTimeline, work: () => Promise<T>): Promise<T> {
		const done = timeline.publishing.then(work);
		timeline.publishing = done.catch(() => undefined);
		return done;
	}

	private async showCall(
		deliveryID: string,
		timeline: DeliveryTimeline,
		addressing: Addressing,
		progress: ToolCallProgress
	): Promise<void> {
		const known = timeline.calls.get(progress.toolCallID);
		const title = progress.title ?? known?.title;
		if (title === undefined) return;
		timeline.calls.set(progress.toolCallID, { title, status: progress.status ?? known?.status ?? 'pending' });
		timeline.addressing = addressing;
		this.settings.work?.working(addressing, { deliveryID, steps: [...timeline.calls.values()] });
		if (!this.settings.postsProgressMessages) return;
		const text = [...timeline.calls.values()].map((call) => `${statusMarkers[call.status]} ${call.title}`).join('\n');
		if (timeline.progressMessageID === undefined) {
			timeline.progressMessageID = await this.settings.postToConversation(addressing, text);
			return;
		}
		await this.settings.editInConversation(addressing, timeline.progressMessageID, text);
	}

	private async sayCheckpoint(
		deliveryID: string,
		timeline: DeliveryTimeline,
		addressing: Addressing,
		checkpoint: string
	): Promise<void> {
		timeline.addressing = addressing;
		const progressMessageID = timeline.progressMessageID;
		if (progressMessageID !== undefined) {
			const isEdited = await this.settings.editInConversation(addressing, progressMessageID, checkpoint).then(
				() => true,
				(failure) => {
					this.settings.report?.(`the progress message ${progressMessageID} did not become the checkpoint: ${String(failure)}`);
					return false;
				}
			);
			if (isEdited) {
				beginNextProgress(timeline);
				this.settings.work?.said(addressing, { deliveryID, messageID: progressMessageID, isStillWorking: true });
				return;
			}
		}
		const messageID = await this.settings.postToConversation(addressing, checkpoint);
		await this.takeDownProgress(timeline, addressing);
		this.settings.work?.said(addressing, { deliveryID, messageID, isStillWorking: true });
	}

	private async takeDownProgress(timeline: DeliveryTimeline, addressing: Addressing): Promise<void> {
		const progressMessageID = timeline.progressMessageID;
		beginNextProgress(timeline);
		if (progressMessageID === undefined) return;
		await this.settings.deleteInConversation(addressing, progressMessageID).catch((failure) => {
			this.settings.report?.(`the progress message ${progressMessageID} was left standing: ${String(failure)}`);
		});
	}

	private finish(deliveryID: string): void {
		this.deliveries.delete(deliveryID);
		this.finishedDeliveryIDs.add(deliveryID);
		if (this.finishedDeliveryIDs.size <= finishedDeliveriesRemembered) return;
		const oldest = this.finishedDeliveryIDs.values().next().value;
		if (oldest !== undefined) this.finishedDeliveryIDs.delete(oldest);
	}
}

function beginNextProgress(timeline: DeliveryTimeline): void {
	timeline.calls = new Map();
	timeline.progressMessageID = undefined;
}
