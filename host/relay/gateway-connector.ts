export type RoutedCall = {
	kind: 'publish';
	requestID: string;
	memberID: string;
	event: Record<string, unknown>;
};

export type GatewayAnswer = {
	kind: 'result';
	requestID: string;
	status: number;
	body: unknown;
};

export type SignedEvent = {
	id: string;
	pubkey: string;
	sig: string;
};

export type BuzzPublisher = {
	publish: (event: Record<string, unknown>) => Promise<void>;
	pubkeyOfMember: (memberID: string) => Promise<string | null>;
};

export function parseRoutedCall(payload: unknown): RoutedCall | null {
	if (typeof payload !== 'object' || payload === null) return null;
	const { kind, requestID, memberID, event } = payload as Record<string, unknown>;
	if (kind !== 'publish') return null;
	if (typeof requestID !== 'string' || requestID.trim() === '') return null;
	if (typeof memberID !== 'string' || memberID.trim() === '') return null;
	if (typeof event !== 'object' || event === null) return null;
	return { kind, requestID, memberID, event: event as Record<string, unknown> };
}

export function signedEventOf(event: Record<string, unknown>): SignedEvent | null {
	const { id, pubkey, sig } = event;
	if (typeof id !== 'string' || id.length !== 64) return null;
	if (typeof pubkey !== 'string' || pubkey.length !== 64) return null;
	if (typeof sig !== 'string' || sig.length !== 128) return null;
	return { id, pubkey, sig };
}

export function answer(requestID: string, status: number, body: unknown): GatewayAnswer {
	return { kind: 'result', requestID, status, body };
}

// The gateway says which member asked; the event says which key signed it. A
// member may only publish what their own key signed, so a stolen or borrowed
// event cannot be posted under someone else's connection.
export async function serveRoutedCall(call: RoutedCall, publisher: BuzzPublisher): Promise<GatewayAnswer> {
	const signed = signedEventOf(call.event);
	if (!signed) return answer(call.requestID, 400, { error: 'that is not a signed event' });

	const theirPubkey = await publisher.pubkeyOfMember(call.memberID);
	if (!theirPubkey) return answer(call.requestID, 403, { error: 'this member has no buzz identity' });
	if (theirPubkey.toLowerCase() !== signed.pubkey.toLowerCase()) {
		return answer(call.requestID, 403, { error: 'that event was signed by another key' });
	}

	try {
		await publisher.publish(call.event);
	} catch (refusal) {
		const reason = refusal instanceof Error ? refusal.message : 'the relay would not take it';
		return answer(call.requestID, 502, { error: reason });
	}
	return answer(call.requestID, 200, { eventID: signed.id });
}
