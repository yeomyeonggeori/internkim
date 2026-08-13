// The envelope is the one the company machine already answers: a capability
// name and a body, replied to by request id. The gateway adds the member,
// which the browser used to assert by attaching its own messenger credential.
export type ClientCall = {
	kind: 'call';
	requestID: string;
	capability: string;
	body: Record<string, unknown>;
};

export type ServerAnswer = {
	kind: 'result';
	requestID: string;
	status: number;
	body: unknown;
};

export type ServerDelivery = {
	kind: 'deliver';
	event: unknown;
	audienceMemberIDs?: string[];
};

export type RoutedCall = {
	kind: 'call';
	requestID: string;
	memberID: string;
	capability: string;
	body: Record<string, unknown>;
};

export const serverOfflineStatus = 503;

export function parseClientCall(payload: unknown): ClientCall | null {
	if (typeof payload !== 'object' || payload === null) return null;
	const { kind, requestID, capability, body } = payload as Record<string, unknown>;
	if (kind !== 'call') return null;
	if (typeof requestID !== 'string' || requestID.trim() === '') return null;
	if (typeof capability !== 'string' || capability.trim() === '') return null;
	if (body !== undefined && (typeof body !== 'object' || body === null)) return null;
	return { kind, requestID, capability, body: (body as Record<string, unknown>) ?? {} };
}

export function parseServerMessage(payload: unknown): ServerAnswer | ServerDelivery | null {
	if (typeof payload !== 'object' || payload === null) return null;
	const record = payload as Record<string, unknown>;
	if (record.kind === 'result') {
		if (typeof record.requestID !== 'string' || record.requestID.trim() === '') return null;
		if (typeof record.status !== 'number') return null;
		return { kind: 'result', requestID: record.requestID, status: record.status, body: record.body };
	}
	if (record.kind === 'deliver') {
		if (typeof record.event !== 'object' || record.event === null) return null;
		return { kind: 'deliver', event: record.event, audienceMemberIDs: memberIDList(record.audienceMemberIDs) };
	}
	return null;
}

function memberIDList(offered: unknown): string[] | undefined {
	if (!Array.isArray(offered)) return undefined;
	const memberIDs = offered.filter((entry): entry is string => typeof entry === 'string' && entry.trim() !== '');
	return memberIDs.length > 0 ? memberIDs : undefined;
}

export function serverOfflineAnswer(requestID: string): ServerAnswer {
	return { kind: 'result', requestID, status: serverOfflineStatus, body: { error: 'server_offline' } };
}

export function routedCallOf(call: ClientCall, memberID: string): RoutedCall {
	return { kind: 'call', requestID: call.requestID, memberID, capability: call.capability, body: call.body };
}

// A reconnecting client resends the call it never saw answered, and the same
// requestID must reach the server once. The ledger holds what an already-seen
// call answered so the repeat is answered rather than forwarded again.
export class CallLedger {
	private readonly answers = new Map<string, ServerAnswer>();
	private readonly pending = new Set<string>();

	constructor(private readonly capacity = 4096) {}

	answerFor(requestID: string): ServerAnswer | undefined {
		return this.answers.get(requestID);
	}

	isPending(requestID: string): boolean {
		return this.pending.has(requestID);
	}

	markPending(requestID: string): void {
		this.pending.add(requestID);
	}

	recordAnswer(answer: ServerAnswer): void {
		this.pending.delete(answer.requestID);
		this.answers.set(answer.requestID, answer);
		this.forgetOldest();
	}

	forgetPending(requestID: string): void {
		this.pending.delete(requestID);
	}

	private forgetOldest(): void {
		while (this.answers.size > this.capacity) {
			const oldest = this.answers.keys().next();
			if (oldest.done) return;
			this.answers.delete(oldest.value);
		}
	}
}

export type CallDecision =
	| { action: 'answer'; answer: ServerAnswer }
	| { action: 'ignore' }
	| { action: 'forward'; routed: RoutedCall };

export function decideCall(
	call: ClientCall,
	memberID: string,
	ledger: CallLedger,
	isServerConnected: boolean
): CallDecision {
	const alreadyAnswered = ledger.answerFor(call.requestID);
	if (alreadyAnswered) return { action: 'answer', answer: alreadyAnswered };
	if (ledger.isPending(call.requestID)) return { action: 'ignore' };
	if (!isServerConnected) return { action: 'answer', answer: serverOfflineAnswer(call.requestID) };
	return { action: 'forward', routed: routedCallOf(call, memberID) };
}
