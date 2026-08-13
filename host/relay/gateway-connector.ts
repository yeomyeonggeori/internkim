import { serveCallForMember, type Dispatch } from './forward';
import { oversizeNotice } from './answer-size';

export type RoutedCall = {
	kind: 'call';
	requestID: string;
	memberID: string;
	capability: string;
	body: Record<string, unknown>;
};

export type GatewayAnswer = {
	kind: 'result';
	requestID: string;
	status: number;
	body: unknown;
};

export function parseRoutedCall(payload: unknown): RoutedCall | null {
	if (typeof payload !== 'object' || payload === null) return null;
	const { kind, requestID, memberID, capability, body } = payload as Record<string, unknown>;
	if (kind !== 'call') return null;
	if (typeof requestID !== 'string' || requestID.trim() === '') return null;
	if (typeof memberID !== 'string' || memberID.trim() === '') return null;
	if (typeof capability !== 'string' || capability.trim() === '') return null;
	if (body !== undefined && (typeof body !== 'object' || body === null)) return null;
	return { kind, requestID, memberID, capability, body: (body as Record<string, unknown>) ?? {} };
}

export function answer(requestID: string, status: number, body: unknown): GatewayAnswer {
	return { kind: 'result', requestID, status, body };
}

// The gateway verified the caller's token before routing, so the member it
// names is the answer to who is asking and nothing in the body is trusted to
// say otherwise.
export async function serveRoutedCall(
	call: RoutedCall,
	dispatch: Dispatch,
	byteCeiling: number
): Promise<GatewayAnswer> {
	try {
		const served = await serveCallForMember(
			dispatch,
			{ callID: call.requestID, capability: call.capability, body: call.body },
			call.memberID
		);
		const tooBig = oversizeNotice({ callID: call.requestID, status: served.status, body: served.body }, byteCeiling);
		if (tooBig) return answer(call.requestID, tooBig.status, tooBig.body);
		return answer(call.requestID, served.status, served.body);
	} catch (refusal) {
		const reason = refusal instanceof Error ? refusal.message : 'the company machine could not do that';
		return answer(call.requestID, 500, { error: reason });
	}
}
