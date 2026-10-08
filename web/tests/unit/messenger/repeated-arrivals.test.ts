import { describe, expect, test } from 'bun:test';
import { createRepeatedArrivalFilter } from '../../../src/lib/repeated-arrivals';

describe('a message announced by both the company computer and the Buzz relay', () => {
	test('is passed on the first time and recognised as a repeat the second time', () => {
		const isRepeated = createRepeatedArrivalFilter(10);
		const arrival = { kind: 'message.arrived', conversationID: 'c1', messageID: 'm1' };
		expect(isRepeated(arrival)).toBe(false);
		expect(isRepeated({ ...arrival })).toBe(true);
	});

	test('leaves other events and arrivals without a message alone', () => {
		const isRepeated = createRepeatedArrivalFilter(10);
		const typing = { kind: 'typing', conversationID: 'c1', messageID: 'm1' };
		const arrivalWithoutMessage = { kind: 'message.arrived', conversationID: 'c1' };
		expect(isRepeated(typing)).toBe(false);
		expect(isRepeated(typing)).toBe(false);
		expect(isRepeated(arrivalWithoutMessage)).toBe(false);
		expect(isRepeated(arrivalWithoutMessage)).toBe(false);
	});

	test('forgets the oldest message once it remembers more than its limit', () => {
		const isRepeated = createRepeatedArrivalFilter(2);
		for (const messageID of ['m1', 'm2', 'm3']) isRepeated({ kind: 'message.arrived', messageID });
		expect(isRepeated({ kind: 'message.arrived', messageID: 'm1' })).toBe(false);
		expect(isRepeated({ kind: 'message.arrived', messageID: 'm3' })).toBe(true);
	});
});
