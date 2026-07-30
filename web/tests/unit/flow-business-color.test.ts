import { describe, expect, test } from 'bun:test';
import { flowBusinessColor } from '../../src/routes/flow/flow-business-color';
import { flowProjectColor } from '../../src/routes/flow/flow-report-colors';

describe('flowBusinessColor', () => {
	test('uses the color the definitions tab shows for that business', () => {
		const categories = ['여명거리', '김인턴'];

		expect(flowBusinessColor('여명거리', categories)).toBe(flowProjectColor(0));
		expect(flowBusinessColor('김인턴', categories)).toBe(flowProjectColor(1));
	});

	test('falls back to a neutral color for a business the definitions do not list', () => {
		expect(flowBusinessColor('사라진 사업', ['여명거리'])).toBe('#64748b');
		expect(flowBusinessColor('', [])).toBe('#64748b');
	});
});
