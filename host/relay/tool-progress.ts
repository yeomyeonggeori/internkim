import type { Addressing, EditInConversation, PostToConversation } from './acp-session';

export const toolCallStartKind = 'tool_call';
export const toolCallUpdateKind = 'tool_call_update';
export const toolCallStatuses = ['pending', 'in_progress', 'completed', 'failed'] as const;

export type ToolCallStatus = (typeof toolCallStatuses)[number];

export type ToolCallProgress = {
	toolCallID: string;
	title?: string;
	status?: ToolCallStatus;
};

export type StartedToolCall = {
	sessionUpdate: typeof toolCallStartKind;
	toolCallId: string;
	title: string;
	status?: ToolCallStatus | null;
};

export type UpdatedToolCall = {
	sessionUpdate: typeof toolCallUpdateKind;
	toolCallId: string;
	status?: ToolCallStatus | null;
};

export function toolCallProgressOf(call: StartedToolCall | UpdatedToolCall): ToolCallProgress {
	const status = call.status ?? undefined;
	if (call.sessionUpdate === toolCallStartKind) return { toolCallID: call.toolCallId, title: call.title, status };
	return { toolCallID: call.toolCallId, status };
}

export type ToolProgressSettings = {
	postToConversation: PostToConversation;
	editInConversation: EditInConversation;
	report?: (line: string) => void;
};

type ShownCall = { title: string; status: ToolCallStatus };

type DeliveryProgress = {
	calls: Map<string, ShownCall>;
	messageID?: string;
	turn: Promise<void>;
};

const statusMarkers: Record<ToolCallStatus, string> = {
	pending: '○',
	in_progress: '◐',
	completed: '✓',
	failed: '✗'
};

export class ToolProgress {
	private readonly settings: ToolProgressSettings;
	private readonly deliveries = new Map<string, DeliveryProgress>();

	constructor(settings: ToolProgressSettings) {
		this.settings = settings;
	}

	async show(deliveryID: string, addressing: Addressing, progress: ToolCallProgress): Promise<void> {
		const delivery = this.deliveries.get(deliveryID) ?? { calls: new Map(), turn: Promise.resolve() };
		const known = delivery.calls.get(progress.toolCallID);
		const title = progress.title ?? known?.title;
		if (title === undefined) return;
		delivery.calls.set(progress.toolCallID, { title, status: progress.status ?? known?.status ?? 'pending' });
		this.deliveries.set(deliveryID, delivery);
		delivery.turn = delivery.turn.then(() => this.publish(delivery, addressing)).catch((failure) => {
			this.settings.report?.(`progress was not shown: ${String(failure)}`);
		});
		await delivery.turn;
	}

	async replaceWith(deliveryID: string, addressing: Addressing, reply: string): Promise<string | undefined> {
		const delivery = this.deliveries.get(deliveryID);
		if (!delivery) return undefined;
		this.deliveries.delete(deliveryID);
		await delivery.turn;
		if (delivery.messageID === undefined) return undefined;
		try {
			await this.settings.editInConversation(addressing, delivery.messageID, reply);
			return delivery.messageID;
		} catch (failure) {
			this.settings.report?.(`the progress message ${delivery.messageID} was not replaced by the reply: ${String(failure)}`);
			return undefined;
		}
	}

	private async publish(delivery: DeliveryProgress, addressing: Addressing): Promise<void> {
		const text = [...delivery.calls.values()].map((call) => `${statusMarkers[call.status]} ${call.title}`).join('\n');
		if (delivery.messageID === undefined) {
			delivery.messageID = await this.settings.postToConversation(addressing, text);
			return;
		}
		await this.settings.editInConversation(addressing, delivery.messageID, text);
	}
}
