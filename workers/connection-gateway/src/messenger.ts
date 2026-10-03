import { base64Of, bytesOfBase64 } from './base64';
import {
	messengerHTTPCapability,
	messengerMediaReadCapability,
	messengerMediaStageCapability,
	messengerMediaWriteCapability
} from './host-protocol';
import type { OneShotAnswer } from './routing';


export const largestMessengerRequestBytes = 1024 * 1024;

export type MessengerCall = (capability: string, body: Record<string, unknown>) => Promise<OneShotAnswer>;

export type RelayResponse = {
	status: number;
	headers: Record<string, string>;
	bodyBase64: string;
	objectURL?: string;
	uploadURL?: string;
	stagedPath?: string;
};

const requestHeadersLeftBehind = new Set([
	'accept-encoding',
	'connection',
	'content-length',
	'cookie',
	'host',
	'keep-alive',
	'proxy-authorization',
	'te',
	'trailer',
	'transfer-encoding',
	'upgrade',
	'x-real-ip'
]);

const responseHeadersLeftBehind = new Set([
	'connection',
	'content-encoding',
	'content-length',
	'keep-alive',
	'trailer',
	'transfer-encoding'
]);

const mediaBlobPath = /^\/media\/[0-9a-f]{64}(\.[a-z0-9.]+)?$/;
const mediaUploadPaths = new Set(['/upload', '/media/upload']);

export function isMediaRead(method: string, pathname: string): boolean {
	return (method === 'GET' || method === 'HEAD') && mediaBlobPath.test(pathname);
}

export function isPageNavigation(request: Request, pathname: string): boolean {
	if (request.method !== 'GET' && request.method !== 'HEAD') return false;
	if (request.headers.has('Upgrade') || isMediaRead(request.method, pathname)) return false;
	return (request.headers.get('Accept') ?? '').includes('text/html');
}

export function isMediaWrite(method: string, pathname: string): boolean {
	return method === 'PUT' && mediaUploadPaths.has(pathname);
}

export function forwardedRequestHeadersOf(headers: Headers): Record<string, string> {
	const forwarded: Record<string, string> = {};
	headers.forEach((value, name) => {
		const lowered = name.toLowerCase();
		if (requestHeadersLeftBehind.has(lowered)) return;
		if (lowered.startsWith('cf-') || lowered.startsWith('x-forwarded-')) return;
		forwarded[lowered] = value;
	});
	return forwarded;
}

export function answeredHeadersOf(headers: Record<string, string>): Headers {
	const answered = new Headers();
	for (const [name, value] of Object.entries(headers)) {
		if (responseHeadersLeftBehind.has(name.toLowerCase())) continue;
		answered.set(name, value);
	}
	return answered;
}

export function relayResponseOf(answer: OneShotAnswer): RelayResponse | null {
	const body = answer.body;
	if (typeof body !== 'object' || body === null) return null;
	const record: Record<string, unknown> = { ...body };
	if (typeof record.bodyBase64 !== 'string') return null;
	return {
		status: answer.status,
		headers: stringRecordOf(record.headers),
		bodyBase64: record.bodyBase64,
		...optionalString('objectURL', record.objectURL),
		...optionalString('uploadURL', record.uploadURL),
		...optionalString('stagedPath', record.stagedPath)
	};
}

function optionalString(name: string, value: unknown): Record<string, string> {
	return typeof value === 'string' && value !== '' ? { [name]: value } : {};
}

function stringRecordOf(value: unknown): Record<string, string> {
	if (typeof value !== 'object' || value === null) return {};
	const strings: Record<string, string> = {};
	for (const [name, entry] of Object.entries(value)) {
		if (typeof entry === 'string') strings[name] = entry;
	}
	return strings;
}

export async function answerMessengerRequest(
	request: Request,
	publicHost: string,
	call: MessengerCall
): Promise<Response> {
	const url = new URL(request.url);
	const target = { method: request.method, path: `${url.pathname}${url.search}`, host: publicHost };
	if (isMediaRead(request.method, url.pathname)) return readMedia(request, target, call);
	if (isMediaWrite(request.method, url.pathname)) return writeMedia(request, target, call);
	return relayHTTP(request, target, call);
}

type Target = { method: string; path: string; host: string };

async function relayHTTP(request: Request, target: Target, call: MessengerCall): Promise<Response> {
	if (Number(request.headers.get('Content-Length') ?? 0) > largestMessengerRequestBytes) {
		return refusal(413, `a messenger request may carry at most ${largestMessengerRequestBytes} bytes`);
	}
	const body = await request.arrayBuffer();
	if (body.byteLength > largestMessengerRequestBytes) {
		return refusal(413, `a messenger request may carry at most ${largestMessengerRequestBytes} bytes`);
	}
	const answer = await call(messengerHTTPCapability, {
		...target,
		headers: forwardedRequestHeadersOf(request.headers),
		bodyBase64: base64Of(new Uint8Array(body))
	});
	return responseOf(answer);
}

async function readMedia(request: Request, target: Target, call: MessengerCall): Promise<Response> {
	const answer = await call(messengerMediaReadCapability, {
		...target,
		headers: forwardedRequestHeadersOf(request.headers)
	});
	const relayed = relayResponseOf(answer);
	if (relayed && request.method === 'HEAD') return headResponseOf(relayed);
	if (!relayed?.objectURL) return responseOf(answer);
	const range = request.headers.get('Range');
	const stored = await fetch(relayed.objectURL, { headers: range ? { Range: range } : {} });
	if (!stored.ok) return refusal(502, `the transfer store answered ${stored.status} for this blob`);
	const headers = answeredHeadersOf(relayed.headers);
	for (const name of ['content-length', 'content-range', 'accept-ranges']) {
		const value = stored.headers.get(name);
		if (value) headers.set(name, value);
	}
	return new Response(stored.body, { status: stored.status, headers });
}

async function writeMedia(request: Request, target: Target, call: MessengerCall): Promise<Response> {
	const staged = relayResponseOf(await call(messengerMediaStageCapability, target));
	if (!staged?.uploadURL || !staged.stagedPath) return refusal(502, 'the company computer gave no place to stage the upload');
	const uploaded = await fetch(staged.uploadURL, {
		method: 'PUT',
		headers: { 'Content-Type': request.headers.get('Content-Type') ?? 'application/octet-stream' },
		body: request.body
	});
	if (!uploaded.ok) return refusal(502, `the transfer store answered ${uploaded.status} for the upload`);
	const answer = await call(messengerMediaWriteCapability, {
		...target,
		headers: forwardedRequestHeadersOf(request.headers),
		stagedPath: staged.stagedPath
	});
	return responseOf(answer);
}

function responseOf(answer: OneShotAnswer): Response {
	const relayed = relayResponseOf(answer);
	if (!relayed) return new Response(JSON.stringify(answer.body), { status: answer.status, headers: { 'Content-Type': 'application/json' } });
	const isBodiless = relayed.status === 204 || relayed.status === 304 || relayed.bodyBase64 === '';
	return new Response(isBodiless ? null : bytesOfBase64(relayed.bodyBase64), {
		status: relayed.status,
		headers: answeredHeadersOf(relayed.headers)
	});
}

function headResponseOf(relayed: RelayResponse): Response {
	const headers = answeredHeadersOf(relayed.headers);
	const length = relayed.headers['content-length'];
	if (length) headers.set('content-length', length);
	return new Response(null, { status: relayed.status, headers });
}

function refusal(status: number, reason: string): Response {
	return new Response(JSON.stringify({ error: reason }), { status, headers: { 'Content-Type': 'application/json' } });
}
