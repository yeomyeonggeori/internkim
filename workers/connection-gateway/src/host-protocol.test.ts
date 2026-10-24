import { describe, expect, test } from 'bun:test';
import { binaryMediaBody, parseBinaryMediaBody, parseGatewayMediaMessage, parseStreamMessage, sendableClose, type MediaOpen } from './host-protocol';

describe('parseStreamMessage', () => {
	test('reads the three messages a stream is made of', () => {
		expect(parseStreamMessage({ kind: 'stream.open', streamID: 's1', host: 'acme.example.test', path: '/' })).toEqual({
			kind: 'stream.open',
			streamID: 's1',
			host: 'acme.example.test',
			path: '/'
		});
		expect(parseStreamMessage({ kind: 'stream.frame', streamID: 's1', data: '[]' })).toEqual({
			kind: 'stream.frame',
			streamID: 's1',
			data: '[]'
		});
		expect(parseStreamMessage({ kind: 'stream.close', streamID: 's1' })).toEqual({
			kind: 'stream.close',
			streamID: 's1',
			code: 1000,
			reason: ''
		});
	});

	test('refuses one naming no stream, no host or no data', () => {
		expect(parseStreamMessage({ kind: 'stream.frame', data: '[]' })).toBeNull();
		expect(parseStreamMessage({ kind: 'stream.open', streamID: 's1' })).toBeNull();
		expect(parseStreamMessage({ kind: 'stream.frame', streamID: 's1', data: 7 })).toBeNull();
	});
});

describe('sendableClose', () => {
	test('keeps a code an endpoint may send', () => {
		expect(sendableClose(4001, 'restricted')).toEqual({ code: 4001, reason: 'restricted' });
		expect(sendableClose(1012, '')).toEqual({ code: 1012, reason: '' });
	});

	test('turns the codes only a runtime reports into ones a socket can send', () => {
		expect(sendableClose(1005, '').code).toBe(1000);
		expect(sendableClose(1006, '').code).toBe(1011);
		expect(sendableClose(1015, '').code).toBe(1011);
	});

	test('cuts a reason to the 123 bytes a close frame carries', () => {
		expect(new TextEncoder().encode(sendableClose(1000, '가'.repeat(60)).reason).byteLength).toBeLessThanOrEqual(123);
	});
});

describe('a binary media body', () => {
	test('carries its stream and every byte, whatever the bytes are', () => {
		const bytes = Uint8Array.from({ length: 70_000 }, (_, index) => index % 256);
		const frame = binaryMediaBody('0f6c3a52-8a1e-4f43-9a3b-1f2d3c4b5a69', bytes);
		expect(parseBinaryMediaBody(frame.buffer)).toEqual({ kind: 'media.body', streamID: '0f6c3a52-8a1e-4f43-9a3b-1f2d3c4b5a69', bytes });
	});

	test('is refused when it names no stream or is cut inside the name', () => {
		expect(parseBinaryMediaBody(new Uint8Array([0, 1, 2]))).toBeNull();
		expect(parseBinaryMediaBody(new Uint8Array([36, 65, 66]))).toBeNull();
		expect(parseBinaryMediaBody(new Uint8Array(0))).toBeNull();
	});

	test('is asked for only by an open that says so, which an older host ignores', () => {
		const open: MediaOpen = { kind: 'media.open', streamID: 's1', method: 'GET', path: '/media/a', host: 'acme.example.test', headers: {}, creditBytes: 1 };
		expect(parseGatewayMediaMessage({ ...open, bodyFrames: 'binary' })).toEqual({ ...open, bodyFrames: 'binary' });
		expect(parseGatewayMediaMessage({ ...open, bodyFrames: 'carrier pigeon' })).toEqual(open);
	});
});
