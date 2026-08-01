import { describe, expect, test } from 'bun:test';
import { orderChannels, moveChannel } from '../../../src/routes/messenger/channel-order';

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
