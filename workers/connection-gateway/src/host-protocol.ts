import { base64Of, bytesOfBase64 } from './base64';

export const largestStreamFrameBytes = 512 * 1024;

export const pingFrame = '{"kind":"ping"}';
export const pongFrame = '{"kind":"pong"}';

export const messengerHTTPCapability = 'messenger.http';
export const messengerMediaReadCapability = 'messenger.media.read';
export const messengerMediaStageCapability = 'messenger.media.stage';
export const messengerMediaWriteCapability = 'messenger.media.write';

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
