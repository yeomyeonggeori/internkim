import { describe, expect, test } from 'bun:test';
import { parseStreamMessage, sendableClose } from './host-protocol';

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
