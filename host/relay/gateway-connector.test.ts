import { describe, expect, test } from 'bun:test';
import { parseRoutedCall, serveRoutedCall, signedEventOf, type BuzzPublisher } from './gateway-connector';

const theirPubkey = 'a'.repeat(64);
const signedEvent = { id: 'b'.repeat(64), pubkey: theirPubkey, sig: 'c'.repeat(128) };
const call = { kind: 'publish' as const, requestID: 'r1', memberID: 'm1', event: signedEvent };

function publisherThat(published: Record<string, unknown>[], pubkey: string | null = theirPubkey): BuzzPublisher {
	return {
		publish: async (event) => {
			published.push(event);
		},
		pubkeyOfMember: async () => pubkey
	};
}

describe('parseRoutedCall', () => {
	test('takes a publish naming a member and an event', () => {
		expect(parseRoutedCall(call)).toEqual(call);
	});

	test('refuses a call missing what it needs', () => {
		expect(parseRoutedCall({ ...call, memberID: '' })).toBeNull();
		expect(parseRoutedCall({ ...call, kind: 'deliver' })).toBeNull();
		expect(parseRoutedCall('publish')).toBeNull();
	});
});

describe('signedEventOf', () => {
	test('refuses anything not carrying a full id, pubkey and signature', () => {
		expect(signedEventOf({ ...signedEvent, sig: 'c'.repeat(64) })).toBeNull();
		expect(signedEventOf({ ...signedEvent, pubkey: 'short' })).toBeNull();
		expect(signedEventOf({ id: signedEvent.id })).toBeNull();
	});
});

describe('serveRoutedCall', () => {
	test('publishes what the asking member signed', async () => {
		const published: Record<string, unknown>[] = [];
		const result = await serveRoutedCall(call, publisherThat(published));
		expect(result).toEqual({ kind: 'result', requestID: 'r1', status: 200, body: { eventID: signedEvent.id } });
		expect(published).toEqual([signedEvent]);
	});

	test('refuses an event another key signed', async () => {
		const published: Record<string, unknown>[] = [];
		const result = await serveRoutedCall(call, publisherThat(published, 'd'.repeat(64)));
		expect(result.status).toBe(403);
		expect(published).toEqual([]);
	});

	test('refuses a member with no buzz identity', async () => {
		const result = await serveRoutedCall(call, publisherThat([], null));
		expect(result.status).toBe(403);
	});

	test('reports a relay that would not take the event', async () => {
		const result = await serveRoutedCall(call, {
			publish: () => Promise.reject(new Error('relay closed the socket')),
			pubkeyOfMember: async () => theirPubkey
		});
		expect(result).toEqual({
			kind: 'result',
			requestID: 'r1',
			status: 502,
			body: { error: 'relay closed the socket' }
		});
	});
});


