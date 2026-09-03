import { describe, expect, test } from 'bun:test';
import { LabelUnresolved, labelOf } from '$lib/server/public-api/record/labels';

const registered = [{ name: '영업' }, { name: '개발', color: '#2563eb' }, { name: '개발지원' }];

function refusalOf(asked: string, over = registered): LabelUnresolved {
	try {
		labelOf(over, asked, null);
	} catch (thrown) {
		if (thrown instanceof LabelUnresolved) return thrown;
		throw thrown;
	}
	throw new Error(`${asked} was expected to be refused`);
}

describe('naming a label', () => {
	test('takes a whole label whatever the case', () => {
		expect(labelOf([{ name: 'Sales' }, { name: 'Dev' }], 'sales', null)).toBe('Sales');
	});

	test('takes a part only one label holds, whatever the case', () => {
		expect(labelOf([{ name: 'Sales' }, { name: 'Dev' }], 'DE', null)).toBe('Dev');
	});

	test('takes a whole label even when a longer one holds it', () => {
		expect(labelOf(registered, '개발', null)).toBe('개발');
	});

	test('keeps what the row held when the caller named nothing', () => {
		expect(labelOf(registered, undefined, '개발')).toBe('개발');
	});

	test('empties the label when the caller named an empty one', () => {
		expect(labelOf(registered, '  ', '개발')).toBeNull();
	});

	test('lets any label through when the company registered none', () => {
		expect(labelOf([], '마케팅', null)).toBe('마케팅');
	});

	test('says a part several labels hold is a question, not a refusal', () => {
		const refusal = refusalOf('발');
		expect(refusal.outcome).toBe('ambiguous');
		expect(refusal.errorCode).toBe('interaction_required');
	});

	test('says a label nothing holds is unregistered, and names what is', () => {
		const refusal = refusalOf('마케팅');
		expect(refusal.outcome).toBe('unregistered');
		expect(refusal.errorCode).toBe('task_label_unregistered');
		expect(refusal.registered).toEqual(['영업', '개발', '개발지원']);
	});
});
