import { describe, expect, test } from 'bun:test';
import { statusIconClass } from '../../src/routes/flow/flow-style';

describe('flow status icon styles', () => {
	const cases: Array<[string, string]> = [
		['완료', 'text-[#16a34a]'],
		['진행', 'text-[#0284c7]'],
		['예정', 'text-[#d97706]'],
		['요청', 'text-[#7c3aed]'],
		['일시정지', 'text-[#e11d48]'],
		['기각', 'text-[#dc2626]'],
		['중단', 'text-[#dc2626]']
	];

	for (const [status, expected] of cases) {
		test(`uses the existing palette for ${status}`, () => {
			expect(statusIconClass(status)).toBe(expected);
		});
	}

	test('uses the muted icon color for an unknown status', () => {
		expect(statusIconClass('알 수 없음')).toBe('text-muted-foreground');
	});
});
