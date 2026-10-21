import { describe, expect, test } from 'bun:test';
import { circlesReading, companyFieldRows } from '../../../src/lib/data-room/company-fields';

const categories = [
	{ code: 'F', parent: null, fields: [] },
	{ code: 'FS', parent: 'F', fields: ['annualRevenue', 'netIncome'] },
	{ code: 'GP', parent: null, fields: ['brandName', 'history'] },
	{ code: 'X', parent: null }
];

describe('the company fields an administrator places', () => {
	test('lists every field with a value, then the ones a category names but nothing holds', () => {
		const rows = companyFieldRows(categories, {
			profile: { brandName: { ko: '샘플' } },
			metrics: [
				{ metric: 'annualRevenue', period: '2024', value: 1 },
				{ metric: 'annualRevenue', period: '2025', value: 2 },
				{ metric: 'burnRate', period: '2025', value: 3 }
			],
			records: [{ category: 'history', title: '설립', attributes: {} }]
		});
		expect(rows).toEqual([
			{ field: 'brandName', kind: 'profile', categoryCode: 'GP' },
			{ field: 'annualRevenue', kind: 'metric', categoryCode: 'FS' },
			{ field: 'burnRate', kind: 'metric', categoryCode: 'X' },
			{ field: 'history', kind: 'record', categoryCode: 'GP' },
			{ field: 'netIncome', kind: 'empty', categoryCode: 'FS' }
		]);
	});

	test('a circle reading a parent sees what its children hold, and not the other way round', () => {
		const circles = [
			{ id: 'finance', readableCategories: ['F'] },
			{ id: 'statements', readableCategories: ['FS'] }
		];
		expect(circlesReading('FS', circles, categories).map((circle) => circle.id)).toEqual(['finance', 'statements']);
		expect(circlesReading('F', circles, categories).map((circle) => circle.id)).toEqual(['finance']);
	});
});
