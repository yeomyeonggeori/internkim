import { describe, expect, test } from 'bun:test';
import { statusOutlineClass } from '../../src/routes/flow/flow-style';

describe('flow status outline styles', () => {
	const cases: Array<[string, string]> = [
		['완료', 'border-[#a8d08d] bg-[#d4edbc]/35 text-[#1f3826]'],
		['진행', 'border-[#8ec7e8] bg-[#bfe1f6]/35 text-[#0b3d63]'],
		['예정', 'border-[#e8c96f] bg-[#ffe5a0]/35 text-[#473821]'],
		['요청', 'border-[#c6a3d9] bg-[#e6cff2]/35 text-[#3d1c52]'],
		['일시정지', 'border-[#e6aaa4] bg-[#ffcfc9]/35 text-[#5b1c14]'],
		['기각', 'border-[#d99a95] bg-[#f6c1bd]/35 text-[#5b1c14]'],
		['중단', 'border-[#d99a95] bg-[#f6c1bd]/35 text-[#5b1c14]']
	];

	for (const [status, expected] of cases) {
		test(`uses the existing palette for ${status}`, () => {
			expect(statusOutlineClass(status)).toBe(expected);
		});
	}

	test('uses the muted outline for an unknown status', () => {
		expect(statusOutlineClass('알 수 없음')).toBe('border-border bg-muted/35 text-muted-foreground');
	});
});
