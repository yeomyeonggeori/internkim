import type { Addressing, DeleteInConversation, EditInConversation, PostToConversation } from './acp-session';

export const startedToolCallKind = 'tool_call';
export const updatedToolCallKind = 'tool_call_update';
export const toolCallStatuses = ['pending', 'in_progress', 'completed', 'failed'] as const;

export type ToolCallStatus = (typeof toolCallStatuses)[number];

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

export type ToolProgressSettings = {
	postToConversation: PostToConversation;
	editInConversation: EditInConversation;
	deleteInConversation: DeleteInConversation;
	report?: (line: string) => void;
};

type ShownCall = { title: string; status: ToolCallStatus };

type DeliveryTimeline = {
	calls: Map<string, ShownCall>;
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
			if (!this.finishedDeliveryIDs.has(deliveryID)) await this.showCall(timeline, addressing, progress);
		};
		await this.inOrder(timeline, show).catch((failure) => {
			this.settings.report?.(`progress was not shown: ${String(failure)}`);
		});
	}

	async showCheckpoint(deliveryID: string, addressing: Addressing, checkpoint: string): Promise<void> {
		const timeline = this.timelineOf(deliveryID);
		await this.inOrder(timeline, () => this.sayCheckpoint(timeline, addressing, checkpoint)).catch((failure) => {
			this.settings.report?.(`the checkpoint was not shown: ${String(failure)}`);
		});
	}

	async postFinal(deliveryID: string, addressing: Addressing, post: () => Promise<string>): Promise<string> {
		const timeline = this.timelineOf(deliveryID);
		return this.inOrder(timeline, async () => {
			const messageID = await post();
			await this.takeDownProgress(timeline, addressing);
			this.finish(deliveryID);
			return messageID;
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

	private async showCall(timeline: DeliveryTimeline, addressing: Addressing, progress: ToolCallProgress): Promise<void> {
		const known = timeline.calls.get(progress.toolCallID);
		const title = progress.title ?? known?.title;
		if (title === undefined) return;
		timeline.calls.set(progress.toolCallID, { title, status: progress.status ?? known?.status ?? 'pending' });
		const text = [...timeline.calls.values()].map((call) => `${statusMarkers[call.status]} ${call.title}`).join('\n');
		if (timeline.progressMessageID === undefined) {
			timeline.progressMessageID = await this.settings.postToConversation(addressing, text);
			return;
		}
		await this.settings.editInConversation(addressing, timeline.progressMessageID, text);
	}

	private async sayCheckpoint(timeline: DeliveryTimeline, addressing: Addressing, checkpoint: string): Promise<void> {
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
				return;
			}
		}
		await this.settings.postToConversation(addressing, checkpoint);
		await this.takeDownProgress(timeline, addressing);
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
