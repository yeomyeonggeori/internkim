import { base64Of, bytesOfBase64 } from './base64';

export const largestStreamFrameBytes = 512 * 1024;

export const pingFrame = '{"kind":"ping"}';
export const pongFrame = '{"kind":"pong"}';

export const messengerHTTPCapability = 'messenger.http';
export const messengerMediaReadCapability = 'messenger.media.read';
export const messengerMediaStageCapability = 'messenger.media.stage';
export const messengerMediaWriteCapability = 'messenger.media.write';
export const messengerMediaStreamCapability = 'messenger.media.stream';

export const hostSupportsHeader = 'x-internkim-host-supports';

export const streamCloseCode = {
	normal: 1000,
	frameTooBig: 1009,
	relayFailed: 1011,
	hostGone: 1012,
	tryAgainLater: 1013
} as const;

export type StreamOpen = {
	kind: 'stream.open';
	streamID: string;
	host: string;
	path: string;
	forwardedFor?: string;
};

export type StreamFrame = {
	kind: 'stream.frame';
	streamID: string;
	data: string;
	isBinary?: true;
};

export type StreamClose = {
	kind: 'stream.close';
	streamID: string;
	code: number;
	reason: string;
};

export type StreamMessage = StreamOpen | StreamFrame | StreamClose;

const streamKinds = new Set(['stream.open', 'stream.frame', 'stream.close']);

export function isStreamKind(kind: unknown): boolean {
	return typeof kind === 'string' && streamKinds.has(kind);
}

export function parseStreamMessage(payload: unknown): StreamMessage | null {
	if (typeof payload !== 'object' || payload === null) return null;
	const record = payload as Record<string, unknown>;
	if (typeof record.streamID !== 'string' || record.streamID === '') return null;
	if (record.kind === 'stream.frame') return parseFrame(record.streamID, record);
	if (record.kind === 'stream.close') return parseClose(record.streamID, record);
	if (record.kind === 'stream.open') return parseOpen(record.streamID, record);
	return null;
}

function parseFrame(streamID: string, record: Record<string, unknown>): StreamFrame | null {
	if (typeof record.data !== 'string') return null;
	if (record.isBinary === true) return { kind: 'stream.frame', streamID, data: record.data, isBinary: true };
	return { kind: 'stream.frame', streamID, data: record.data };
}

function parseClose(streamID: string, record: Record<string, unknown>): StreamClose {
	const code = typeof record.code === 'number' ? record.code : streamCloseCode.normal;
	const reason = typeof record.reason === 'string' ? record.reason : '';
	return { kind: 'stream.close', streamID, code, reason };
}

function parseOpen(streamID: string, record: Record<string, unknown>): StreamOpen | null {
	if (typeof record.host !== 'string' || record.host === '') return null;
	const path = typeof record.path === 'string' && record.path.startsWith('/') ? record.path : '/';
	const forwardedFor = typeof record.forwardedFor === 'string' && record.forwardedFor !== '' ? record.forwardedFor : undefined;
	return { kind: 'stream.open', streamID, host: record.host, path, ...(forwardedFor ? { forwardedFor } : {}) };
}

export function streamFrame(streamID: string, data: string): string {
	return JSON.stringify({ kind: 'stream.frame', streamID, data } satisfies StreamFrame);
}

export function binaryStreamFrame(streamID: string, bytes: Uint8Array): string {
	return JSON.stringify({ kind: 'stream.frame', streamID, data: base64Of(bytes), isBinary: true } satisfies StreamFrame);
}

export function bytesOfBinaryFrame(frame: StreamFrame): Uint8Array<ArrayBuffer> {
	return bytesOfBase64(frame.data);
}

export function streamClose(streamID: string, code: number, reason: string): string {
	return JSON.stringify({ kind: 'stream.close', streamID, code, reason } satisfies StreamClose);
}

export function streamOpen(open: Omit<StreamOpen, 'kind'>): string {
	return JSON.stringify({ kind: 'stream.open', ...open } satisfies StreamOpen);
}

const encoder = new TextEncoder();

export function isBinaryFrameTooBig(bytes: ArrayBuffer | Uint8Array): boolean {
	return bytes.byteLength > largestStreamFrameBytes;
}

export function isFrameTooBig(data: string): boolean {
	if (data.length > largestStreamFrameBytes) return true;
	if (data.length * 3 <= largestStreamFrameBytes) return false;
	return encoder.encode(data).byteLength > largestStreamFrameBytes;
}

const largestCloseReasonBytes = 123;

export function closeReasonThatFits(reason: string): string {
	let fitted = reason;
	while (encoder.encode(fitted).byteLength > largestCloseReasonBytes) fitted = fitted.slice(0, -1);
	return fitted;
}

export function sendableClose(code: number, reason: string): { code: number; reason: string } {
	return { code: sendableCloseCode(code), reason: closeReasonThatFits(reason) };
}

function sendableCloseCode(code: number): number {
	if (code === 1005) return streamCloseCode.normal;
	if (code === 1000 || (code >= 3000 && code <= 4999)) return code;
	if (code >= 1001 && code <= 1014 && code !== 1004 && code !== 1006) return code;
	return streamCloseCode.relayFailed;
}

export const mediaCreditWindowBytes = 512 * 1024;

export type MediaOpen = {
	kind: 'media.open';
	streamID: string;
	method: string;
	path: string;
	host: string;
	headers: Record<string, string>;
	creditBytes: number;
};

export type MediaCredit = { kind: 'media.credit'; streamID: string; bytes: number };
export type MediaStop = { kind: 'media.stop'; streamID: string; reason: string };
export type MediaHead = { kind: 'media.head'; streamID: string; status: number; headers: Record<string, string> };
export type MediaBody = { kind: 'media.body'; streamID: string; data: string };
export type MediaEnd = { kind: 'media.end'; streamID: string };
export type MediaFail = { kind: 'media.fail'; streamID: string; reason: string };

export type GatewayMediaMessage = MediaOpen | MediaCredit | MediaStop;
export type HostMediaMessage = MediaHead | MediaBody | MediaEnd | MediaFail;
export type MediaMessage = GatewayMediaMessage | HostMediaMessage;

const mediaKinds = new Set(['media.open', 'media.credit', 'media.stop', 'media.head', 'media.body', 'media.end', 'media.fail']);

export function isMediaKind(kind: unknown): boolean {
	return typeof kind === 'string' && mediaKinds.has(kind);
}

export function parseHostMediaMessage(payload: unknown): HostMediaMessage | null {
	const message = parseMediaMessage(payload);
	if (message?.kind === 'media.open' || message?.kind === 'media.credit' || message?.kind === 'media.stop') return null;
	return message;
}

export function parseGatewayMediaMessage(payload: unknown): GatewayMediaMessage | null {
	const message = parseMediaMessage(payload);
	if (message?.kind === 'media.open' || message?.kind === 'media.credit' || message?.kind === 'media.stop') return message;
	return null;
}

function parseMediaMessage(payload: unknown): MediaMessage | null {
	if (typeof payload !== 'object' || payload === null) return null;
	const record: Record<string, unknown> = { ...payload };
	const streamID = record.streamID;
	if (typeof streamID !== 'string' || streamID === '') return null;
	if (record.kind === 'media.open') return parseMediaOpen(streamID, record);
	if (record.kind === 'media.credit') return parseMediaCredit(streamID, record);
	if (record.kind === 'media.head') return parseMediaHead(streamID, record);
	if (record.kind === 'media.body') return typeof record.data === 'string' ? { kind: 'media.body', streamID, data: record.data } : null;
	if (record.kind === 'media.end') return { kind: 'media.end', streamID };
	if (record.kind === 'media.stop' || record.kind === 'media.fail') return { kind: record.kind, streamID, reason: stringOr(record.reason, '') };
	return null;
}

function parseMediaOpen(streamID: string, record: Record<string, unknown>): MediaOpen | null {
	const { method, path, host, creditBytes } = record;
	if (typeof method !== 'string' || method === '') return null;
	if (typeof path !== 'string' || !path.startsWith('/')) return null;
	if (typeof host !== 'string' || host === '') return null;
	if (!isPositiveByteCount(creditBytes)) return null;
	return { kind: 'media.open', streamID, method, path, host, headers: stringRecordOf(record.headers), creditBytes };
}

function parseMediaCredit(streamID: string, record: Record<string, unknown>): MediaCredit | null {
	return isPositiveByteCount(record.bytes) ? { kind: 'media.credit', streamID, bytes: record.bytes } : null;
}

function parseMediaHead(streamID: string, record: Record<string, unknown>): MediaHead | null {
	if (typeof record.status !== 'number' || !Number.isInteger(record.status)) return null;
	return { kind: 'media.head', streamID, status: record.status, headers: stringRecordOf(record.headers) };
}

function isPositiveByteCount(value: unknown): value is number {
	return typeof value === 'number' && Number.isInteger(value) && value > 0;
}

function stringOr(value: unknown, fallback: string): string {
	return typeof value === 'string' ? value : fallback;
}

function stringRecordOf(value: unknown): Record<string, string> {
	if (typeof value !== 'object' || value === null) return {};
	return Object.fromEntries(Object.entries(value).filter((entry): entry is [string, string] => typeof entry[1] === 'string'));
}

export function mediaMessage(message: MediaMessage): string {
	return JSON.stringify(message);
}

export function mediaBody(streamID: string, bytes: Uint8Array): string {
	return mediaMessage({ kind: 'media.body', streamID, data: base64Of(bytes) });
}
