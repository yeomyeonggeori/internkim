import {
	apiFileCapability,
	apiRequestCapability,
	serveCallForMember,
	servePublicAPIFile,
	servePublicAPIRequest,
	serveTelling,
	tellCapability,
	type Dispatch
} from './forward';
import { oversizeNotice } from './answer-size';
import { messengerCapabilities } from './messenger-calls';

export type RoutedCall = {
	kind: 'call';
	requestID: string;
	memberID: string | null;
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
	if (memberID !== undefined && (typeof memberID !== 'string' || memberID.trim() === '')) return null;
	if (typeof capability !== 'string' || capability.trim() === '') return null;
	if (body !== undefined && (typeof body !== 'object' || body === null)) return null;
	return {
		kind,
		requestID,
		memberID: typeof memberID === 'string' ? memberID : null,
		capability,
		body: (body as Record<string, unknown>) ?? {}
	};
}

export function answer(requestID: string, status: number, body: unknown): GatewayAnswer {
	return { kind: 'result', requestID, status, body };
}

export async function serveRoutedCall(
	call: RoutedCall,
	dispatch: Dispatch,
	byteCeiling: number
): Promise<GatewayAnswer> {
	try {
		const served = await servedCall(call, dispatch);
		const tooBig = oversizeNotice({ callID: call.requestID, status: served.status, body: served.body }, byteCeiling);
		if (tooBig) return answer(call.requestID, tooBig.status, tooBig.body);
		return answer(call.requestID, served.status, served.body);
	} catch (refusal) {
		const reason = refusal instanceof Error ? refusal.message : 'the company machine could not do that';
		return answer(call.requestID, 500, { error: reason });
	}
}

const capabilitiesTheGatewayCarriesItself = new Set([
	apiRequestCapability,
	apiFileCapability,
	tellCapability,
	...messengerCapabilities
]);

async function servedCall(
	call: RoutedCall,
	dispatch: Dispatch
): Promise<{ status: number; body: unknown }> {
	if (capabilitiesTheGatewayCarriesItself.has(call.capability)) {
		if (call.memberID) {
			return {
				status: 403,
				body: { error: `${call.capability} names its own requester, so it is carried by the gateway itself, never over a member connection` }
			};
		}
		if (call.capability === apiRequestCapability) return servePublicAPIRequest(dispatch, call.body);
		if (call.capability === apiFileCapability) return servePublicAPIFile(dispatch, call.body);
		if (messengerCapabilities.has(call.capability)) return dispatch.serveMessenger(call.capability, call.body);
		return serveTelling(dispatch, call.body);
	}
	if (!call.memberID) {
		return { status: 400, body: { error: `${call.capability} is asked for by a member, and none was named` } };
	}
	return serveCallForMember(
		dispatch,
		{ callID: call.requestID, capability: call.capability, body: call.body },
		call.memberID
	);
}
