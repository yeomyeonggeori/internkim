import { WorkerEntrypoint } from 'cloudflare:workers';
import {
	JSONWebKeyCache,
	TokenRefused,
	companyOfHost,
	companyOfMessengerAddress,
	resolveMember,
	verifyToken
} from './identity';
import type { OneShotAnswer, OneShotCall } from './routing';
import {
	digestOf,
	digestsMatch,
	jsonResponse,
	memberHeader,
	streamAddressHeader,
	streamHostHeader,
	streamPathHeader
} from './company-connection';
import { answerMessengerRequest, isPageNavigation } from './messenger';
import { MessengerAddresses } from './messenger-address';

export type WorkerEnvironment = {
	COMPANY_CONNECTIONS: DurableObjectNamespace;
	SUPABASE_URL: string;
	SUPABASE_PUBLISHABLE_KEY: string;
	SUPABASE_JWKS_URL?: string;
	GATEWAY_ADMIN_TOKEN?: string;
};

export default {
	fetch(request: Request, environment: WorkerEnvironment): Promise<Response> {
		return route(request, environment);
	}
} satisfies ExportedHandler<WorkerEnvironment>;

async function route(request: Request, environment: WorkerEnvironment): Promise<Response> {
	const url = new URL(request.url);
	const path = url.pathname.split('/').filter(Boolean);
	if (path[0] !== companyPathSegment) return reachTheMessenger(request, environment, url);
	if (path.length !== 3) return jsonResponse({ error: 'not found' }, 404);
	const companyID = decodeURIComponent(path[1]);

	if (path[2] === 'client') return joinAsClient(request, environment, companyID);
	if (path[2] === 'host') return joinAsHost(request, environment, companyID);
	if (path[2] === 'server') return joinAsServer(request, environment, companyID);
	if (path[2] === 'server-key') return storeServerKey(request, environment, companyID);
	if (path[2] === 'call') return takeCompanyCall(request, environment, companyID);
	return jsonResponse({ error: 'not found' }, 404);
}

const companyPathSegment = 'company';

async function reachTheMessenger(request: Request, environment: WorkerEnvironment, url: URL): Promise<Response> {
	const slug = messengerSlugOf(url.hostname);
	if (!slug) return jsonResponse({ error: 'not found' }, 404);
	if (isPageNavigation(request, url.pathname)) return Response.redirect(theZoneAddressOf(url), 308);
	const companyID = await messengerAddressesFor(environment).companyOf(slug);
	if (!companyID) return jsonResponse({ error: 'no company messenger answers at this address' }, 404);
	if (request.headers.get('Upgrade') === 'websocket') return openMessengerStream(request, environment, companyID, url);
	return answerMessengerRequest(request, url.host, (capability, body) =>
		callTheMessenger(environment, companyID, capability, body)
	);
}

function theZoneAddressOf(url: URL): string {
	const zone = url.hostname.split('.').slice(1).join('.');
	return `https://${zone}${url.pathname}${url.search}`;
}

function messengerSlugOf(hostname: string): string | null {
	const labels = hostname.toLowerCase().split('.');
	if (labels.length < 3) return null;
	return labels[0] || null;
}

let sharedMessengerAddresses: MessengerAddresses | undefined;

function messengerAddressesFor(environment: WorkerEnvironment): MessengerAddresses {
	sharedMessengerAddresses ??= new MessengerAddresses((slug) =>
		companyOfMessengerAddress(environment.SUPABASE_URL, environment.SUPABASE_PUBLISHABLE_KEY, slug)
	);
	return sharedMessengerAddresses;
}

function openMessengerStream(
	request: Request,
	environment: WorkerEnvironment,
	companyID: string,
	url: URL
): Promise<Response> {
	const headers: Record<string, string> = {
		...headersOf(request),
		[streamHostHeader]: url.host,
		[streamPathHeader]: `${url.pathname}${url.search}`
	};
	const address = request.headers.get('CF-Connecting-IP');
	if (address) headers[streamAddressHeader] = address;
	return connectionFor(environment, companyID).fetch(
		new Request(`https://connection-gateway/company/${encodeURIComponent(companyID)}/stream-session`, { headers })
	);
}

async function callTheMessenger(
	environment: WorkerEnvironment,
	companyID: string,
	capability: string,
	body: Record<string, unknown>
): Promise<OneShotAnswer> {
	const answered = await connectionFor(environment, companyID).fetch(
		new Request(`https://connection-gateway/company/${encodeURIComponent(companyID)}/messenger-call`, {
			method: 'POST',
			body: JSON.stringify({ requestID: crypto.randomUUID(), capability, body })
		})
	);
	return oneShotAnswerOfResponse(await answered.json());
}

function oneShotAnswerOfResponse(document: unknown): OneShotAnswer {
	if (typeof document !== 'object' || document === null) return { requestID: '', status: 502, body: null };
	const record: Record<string, unknown> = { ...document };
	return {
		requestID: typeof record.requestID === 'string' ? record.requestID : '',
		status: typeof record.status === 'number' ? record.status : 502,
		body: record.body
	};
}

async function joinAsClient(
	request: Request,
	environment: WorkerEnvironment,
	companyID: string
): Promise<Response> {
	const token = bearerOf(request) ?? tokenOfferedByBrowser(request);
	if (!token) return jsonResponse({ error: 'this call carried no token' }, 401);
	try {
		const claims = await verifyToken(
			token,
			keyCacheFor(environment),
			issuerOf(environment),
			Math.floor(Date.now() / 1000)
		);
		const identity = await resolveMember(
			environment.SUPABASE_URL,
			environment.SUPABASE_PUBLISHABLE_KEY,
			token,
			claims.sub
		);
		if (identity.companyID !== companyID) {
			return jsonResponse({ error: 'this account belongs to another company' }, 403);
		}
		return connectionFor(environment, companyID).fetch(
			new Request(request.url, { headers: { ...headersOf(request), [memberHeader]: identity.memberID } })
		);
	} catch (refusal) {
		if (refusal instanceof TokenRefused) return jsonResponse({ error: refusal.message }, 401);
		throw refusal;
	}
}

function joinAsServer(
	request: Request,
	environment: WorkerEnvironment,
	companyID: string
): Promise<Response> {
	return connectionFor(environment, companyID).fetch(request);
}

async function joinAsHost(
	request: Request,
	environment: WorkerEnvironment,
	companyID: string
): Promise<Response> {
	const token = bearerOf(request);
	if (!token) return jsonResponse({ error: 'this call carried no token' }, 401);
	try {
		const claims = await verifyToken(
			token,
			keyCacheFor(environment),
			issuerOf(environment),
			Math.floor(Date.now() / 1000)
		);
		if (claims.hostCompanyID !== companyID) {
			return jsonResponse({ error: 'this token is not the company host' }, 403);
		}
		if ((await companyOfHost(environment.SUPABASE_URL, environment.SUPABASE_PUBLISHABLE_KEY, token)) !== companyID) {
			return jsonResponse({ error: 'this computer is no longer the company host' }, 403);
		}
		return connectionFor(environment, companyID).fetch(
			new Request(`https://connection-gateway/company/${encodeURIComponent(companyID)}/host-session`, {
				method: request.method,
				headers: headersOf(request)
			})
		);
	} catch (refusal) {
		if (refusal instanceof TokenRefused) return jsonResponse({ error: refusal.message }, 401);
		throw refusal;
	}
}

async function holdsTheGatewayToken(request: Request, environment: WorkerEnvironment): Promise<boolean> {
	const expected = environment.GATEWAY_ADMIN_TOKEN;
	if (!expected) return false;
	return digestsMatch(await digestOf(bearerOf(request) ?? ''), await digestOf(expected));
}

async function takeCompanyCall(
	request: Request,
	environment: WorkerEnvironment,
	companyID: string
): Promise<Response> {
	if (!(await holdsTheGatewayToken(request, environment))) {
		return jsonResponse({ error: 'this call may not speak to a company' }, 401);
	}
	return connectionFor(environment, companyID).fetch(request);
}

async function storeServerKey(
	request: Request,
	environment: WorkerEnvironment,
	companyID: string
): Promise<Response> {
	if (!(await holdsTheGatewayToken(request, environment))) {
		return jsonResponse({ error: 'this call may not set a server key' }, 401);
	}
	return connectionFor(environment, companyID).fetch(request);
}

function issuerOf(environment: WorkerEnvironment): string {
	return `${environment.SUPABASE_URL.replace(/\/+$/, '')}/auth/v1`;
}

let sharedKeyCache: JSONWebKeyCache | undefined;

function keyCacheFor(environment: WorkerEnvironment): JSONWebKeyCache {
	sharedKeyCache ??= new JSONWebKeyCache(
		environment.SUPABASE_JWKS_URL ?? `${issuerOf(environment)}/.well-known/jwks.json`
	);
	return sharedKeyCache;
}

function connectionFor(environment: WorkerEnvironment, companyID: string): DurableObjectStub {
	const namespace = environment.COMPANY_CONNECTIONS;
	return namespace.get(namespace.idFromName(companyID));
}

function bearerOf(request: Request): string | null {
	const offered = request.headers.get('Authorization') ?? '';
	return offered.startsWith('Bearer ') ? offered.slice('Bearer '.length).trim() || null : null;
}

const browserTokenProtocol = 'internkim.bearer.';

// A browser cannot put a header on a websocket handshake, so it carries the
// token as a subprotocol instead. The chosen protocol has to be echoed back or
// the browser closes the connection it just opened.
function tokenOfferedByBrowser(request: Request): string | null {
	const offered = request.headers.get('Sec-WebSocket-Protocol') ?? '';
	for (const protocol of offered.split(',')) {
		const trimmed = protocol.trim();
		if (trimmed.startsWith(browserTokenProtocol)) return trimmed.slice(browserTokenProtocol.length) || null;
	}
	return null;
}

function headersOf(request: Request): Record<string, string> {
	const headers: Record<string, string> = {};
	request.headers.forEach((value, name) => {
		headers[name] = value;
	});
	return headers;
}

// A Cloudflare WorkerEntrypoint is reachable only through a service binding.
export class CompanyCalls extends WorkerEntrypoint<WorkerEnvironment> {
	async callCompany(companyID: string, call: OneShotCall): Promise<OneShotAnswer> {
		const answered = await connectionFor(this.env, companyID).fetch(
			new Request(`https://connection-gateway/company/${encodeURIComponent(companyID)}/call`, {
				method: 'POST',
				body: JSON.stringify(call)
			})
		);
		if (!answered.ok) throw new TypeError('a company call names a requestID and a capability');
		return (await answered.json()) as OneShotAnswer;
	}
}

export { CompanyConnectionObject } from './company-connection';
