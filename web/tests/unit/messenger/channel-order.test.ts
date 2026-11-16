import { describe, expect, test } from 'bun:test';
import { orderChannels, orderDirectMessages, moveChannel, pinnedChannelsOf } from '../../../src/routes/messenger/channel-order';

const channel = (id: string, name: string) => ({ id, name, kind: 'group' as const });

describe('orderChannels default rule', () => {
	test('pins 광장·잡담 first and 일정·업무·근태 last, others in between', () => {
		const input = [
			channel('c1', '근태'),
			channel('c2', '잡담'),
			channel('c3', '자유'),
			channel('c4', '광장'),
			channel('c5', '업무')
		];
		expect(orderChannels(input, []).map((c) => c.name)).toEqual([
			'광장',
			'잡담',
			'자유',
			'업무',
			'근태'
		]);
	});

	test('user order wins over the default rule', () => {
		const input = [channel('c1', '광장'), channel('c2', '근태')];
		expect(orderChannels(input, ['c2', 'c1']).map((c) => c.name)).toEqual(['근태', '광장']);
	});

	test('channels absent from the user order fall after ordered ones by default rank', () => {
		const input = [channel('c1', '광장'), channel('c2', '잡담'), channel('c3', '근태')];
		expect(orderChannels(input, ['c3']).map((c) => c.id)).toEqual(['c3', 'c1', 'c2']);
	});
});

describe('moveChannel', () => {
	test('moves dragged id to the target position', () => {
		expect(moveChannel(['a', 'b', 'c', 'd'], 'd', 'b')).toEqual(['a', 'd', 'b', 'c']);
	});
	test('no-ops on unknown or identical ids', () => {
		expect(moveChannel(['a', 'b'], 'a', 'a')).toEqual(['a', 'b']);
		expect(moveChannel(['a', 'b'], 'x', 'a')).toEqual(['a', 'b']);
	});
});

describe('pinnedChannelsOf', () => {
	test('lists the pinned channels in the order they were pinned and skips one the reader no longer sees', () => {
		const input = [channel('c1', '광장'), channel('c2', '잡담'), channel('c3', '근태')];
		expect(pinnedChannelsOf(input, ['c3', 'gone', 'c1']).map((c) => c.id)).toEqual(['c3', 'c1']);
	});
});

describe('orderDirectMessages', () => {
	const person = (id: string, lastMessageAt?: string) => ({ id, name: id, kind: 'dm' as const, lastMessageAt });

	test('the conversation with the agent always leads, even when another spoke more recently', () => {
		const agent = { id: 'agent', name: '김인턴', kind: 'dm' as const, isWithTheAgent: true, lastMessageAt: '2026-11-01T00:00:00.000Z' };
		const ordered = orderDirectMessages([person('a', '2026-11-16T09:00:00.000Z'), agent]);
		expect(ordered.map((conversation) => conversation.id)).toEqual(['agent', 'a']);
	});

	test('the rest run newest message first, and one with no message yet sinks to the bottom', () => {
		const ordered = orderDirectMessages([
			person('quiet'),
			person('older', '2026-11-15T09:00:00.000Z'),
			person('newest', '2026-11-16T09:00:00.000Z')
		]);
		expect(ordered.map((conversation) => conversation.id)).toEqual(['newest', 'older', 'quiet']);
	});
});
