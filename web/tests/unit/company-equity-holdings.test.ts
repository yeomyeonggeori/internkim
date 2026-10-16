import { describe, expect, test } from 'bun:test';
import { holdingsOf } from '../../src/lib/server/public-api/record/company-equity-tools';

const line = (holder: string, holderKind: 'founder' | 'investor' | 'option_pool' | 'employee', shares: number, securityClass = 'common') => ({
	entryID: `${holder}-${shares}`,
	effectiveDate: '2026-01-01',
	holder,
	holderKind,
	securityClass,
	shares,
	fundingRecordID: null,
	note: null
});

describe('the equity ledger', () => {
	test('holds what each holder of each class has summed, largest first, with its share of the total', () => {
		expect(holdingsOf([
			line('이샘플', 'founder', 700),
			line('스톡옵션 풀', 'option_pool', 100, 'options'),
			line('샘플벤처스', 'investor', 200, 'RCPS'),
			line('스톡옵션 풀', 'option_pool', -40, 'options'),
			line('박예시', 'employee', 40, 'options')
		])).toEqual({
			totalShares: 1000,
			holdings: [
				{ holder: '이샘플', holderKind: 'founder', securityClass: 'common', shares: 700, percent: 70 },
				{ holder: '샘플벤처스', holderKind: 'investor', securityClass: 'RCPS', shares: 200, percent: 20 },
				{ holder: '스톡옵션 풀', holderKind: 'option_pool', securityClass: 'options', shares: 60, percent: 6 },
				{ holder: '박예시', holderKind: 'employee', securityClass: 'options', shares: 40, percent: 4 }
			]
		});
	});

	test('drops a holder whose lines cancel out', () => {
		expect(holdingsOf([line('견본캐피탈', 'investor', 50), line('견본캐피탈', 'investor', -50)]).holdings).toEqual([]);
	});
});
