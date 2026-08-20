import { describe, expect, test } from 'bun:test';
import { donutValueTextClass } from '../../../src/routes/crm/crm-kpi-value-text';
import { formatMoney, formatMoneyTotals } from '../../../src/routes/crm/crm-money';
import { interimCurrencyCatalogue } from '../../../src/lib/currency/currency-catalogue';

describe('donutValueTextClass', () => {
	test('keeps the largest size for a value the donut fits', () => {
		expect(donutValueTextClass('3')).toContain('text-xl');
		expect(donutValueTextClass(formatMoney(8000000, 'KRW', interimCurrencyCatalogue))).toContain('text-xl');
		expect(donutValueTextClass('금액 없음')).toContain('text-xl');
	});

	test('steps down for the amounts that used to be cut off', () => {
		expect(donutValueTextClass(formatMoney(18000000, 'KRW', interimCurrencyCatalogue))).toContain('text-base');
		expect(donutValueTextClass(formatMoney(93000000, 'KRW', interimCurrencyCatalogue))).toContain('text-base');
	});

	test('steps down again for a multi-currency total', () => {
		const totals = formatMoneyTotals({ KRW: 93000000, USD: 12000 }, interimCurrencyCatalogue);
		expect(totals).toBe('₩9,300만 · $12K');
		expect(donutValueTextClass(totals)).toContain('text-xs');
	});
});
