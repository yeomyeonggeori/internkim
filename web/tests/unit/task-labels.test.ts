import { describe, expect, test } from 'bun:test';
import {
	decidedTaskLabelsOf,
	missesATaskLabel,
	withDecidedTaskLabels
} from '../../src/lib/server/public-api/record/task-labels';

type AskedTask = { title: string; business?: string; type?: string; size?: string };

describe('task labels the company decides', () => {
	test('fill only the labels a new task left out', () => {
		const asked: AskedTask = { title: '제안서', size: 'S' };
		const written = withDecidedTaskLabels(asked, { business: '영업', type: '문서', size: 'L' });
		expect(written).toEqual({ title: '제안서', size: 'S', business: '영업', type: '문서' });
	});

	test('leave a label out when the company could decide none for it', () => {
		const asked: AskedTask = { title: '제안서' };
		const written = withDecidedTaskLabels(asked, { business: '', type: '', size: '' });
		expect(written).toEqual({ title: '제안서', business: undefined, type: '', size: undefined });
	});

	test('leave the task as asked when no decision came back', () => {
		expect(withDecidedTaskLabels({ title: '제안서' }, null)).toEqual({ title: '제안서' });
	});

	test('are asked for only while a label is missing', () => {
		expect(missesATaskLabel({ business: '영업', type: '', size: 'M' })).toBe(false);
		expect(missesATaskLabel({ business: '영업', type: '문서' })).toBe(true);
	});

	test('refuse an answer shaped unlike a decision', () => {
		expect(decidedTaskLabelsOf({ business: '영업', type: '문서', size: 'HUGE' })).toBeNull();
		expect(decidedTaskLabelsOf('M')).toBeNull();
		expect(decidedTaskLabelsOf({ business: '영업', type: '', size: 'M' })).toEqual({ business: '영업', type: '', size: 'M' });
	});
});
