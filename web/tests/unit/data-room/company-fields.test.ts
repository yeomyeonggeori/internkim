import { describe, expect, test } from 'bun:test';
import { circlesReading, companyFieldRows } from '../../../src/lib/data-room/company-fields';
import { companyFieldName, fieldNamer } from '../../../src/lib/data-room/company-field-name';
import { dataRoomCompanyText } from '../../../src/lib/data-room/company-text';

const categories = [
	{ code: 'F', parent: null },
	{ code: 'FS', parent: 'F' },
	{ code: 'GP', parent: null },
	{ code: 'X', parent: null }
];

const placed = [
	{ kind: 'metric' as const, field: 'annualRevenue', categoryCode: 'FS', names: null, isPublic: false },
	{ kind: 'metric' as const, field: 'netIncome', categoryCode: 'FS', names: null, isPublic: false },
	{ kind: 'profile' as const, field: 'brandName', categoryCode: 'GP', names: null, isPublic: false },
	{ kind: 'record' as const, field: 'history', categoryCode: 'GP', names: null, isPublic: false },
	{ kind: 'metric' as const, field: 'installedUnits', categoryCode: null, names: { ko: '누적 설치 대수' }, isPublic: false },
	{ kind: 'profile' as const, field: 'slogan', categoryCode: null, names: null, isPublic: true }
];

describe('the company fields an administrator places', () => {
	test('lists every field with a value, then the ones placed or published that nothing holds', () => {
		const rows = companyFieldRows(placed, {
			profile: { brandName: { ko: '샘플' } },
			metrics: [
				{ metric: 'annualRevenue', period: '2024', value: 1, basis: 'reported' },
				{ metric: 'annualRevenue', period: '2025', value: 2, basis: 'reported' },
				{ metric: 'burnRate', period: '2025', value: 3, basis: 'reported' },
				{ metric: 'installedUnits', period: '2025', value: 4, basis: 'reported' }
			],
			records: [{ category: 'history', title: '설립', attributes: {} }]
		});
		expect(rows).toEqual([
			{ kind: 'profile', field: 'brandName', group: 'profile', categoryCode: 'GP', names: null, isPublic: false },
			{ kind: 'metric', field: 'annualRevenue', group: 'metric', categoryCode: 'FS', names: null, isPublic: false },
			{ kind: 'metric', field: 'burnRate', group: 'metric', categoryCode: 'X', names: null, isPublic: false },
			{ kind: 'metric', field: 'installedUnits', group: 'metric', categoryCode: 'X', names: { ko: '누적 설치 대수' }, isPublic: false },
			{ kind: 'record', field: 'history', group: 'record', categoryCode: 'GP', names: null, isPublic: false },
			{ kind: 'metric', field: 'netIncome', group: 'empty', categoryCode: 'FS', names: null, isPublic: false },
			{ kind: 'profile', field: 'slogan', group: 'empty', categoryCode: 'X', names: null, isPublic: true }
		]);
	});

	test('a metric and a record of one name are two fields, each in its own place', () => {
		const rows = companyFieldRows([{ kind: 'record', field: 'history', categoryCode: 'GP', names: null, isPublic: false }], {
			profile: {},
			metrics: [{ metric: 'history', period: '2025', value: 1, basis: 'reported' }],
			records: [{ category: 'history', title: '설립', attributes: {} }]
		});
		expect(rows.map((row) => [row.kind, row.categoryCode])).toEqual([['metric', 'X'], ['record', 'GP']]);
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

describe('the name a company page field is shown under', () => {
	const text = dataRoomCompanyText.ko;

	test("the company's own name comes first, then the product's, then the key as written", () => {
		expect(companyFieldName('metric', 'annualRevenue', { ko: '연 매출액' }, text, 'ko')).toBe('연 매출액');
		expect(companyFieldName('metric', 'annualRevenue', { en: 'Revenue' }, text, 'ko')).toBe('매출');
		expect(companyFieldName('metric', 'installedUnits', null, text, 'ko')).toBe('Installed Units');
	});

	test('a name given to a metric does not rename a record of the same key', () => {
		const fieldName = fieldNamer([{ kind: 'metric', field: 'history', names: { ko: '이력 지수' } }], text, 'ko');
		expect(fieldName('metric', 'history')).toBe('이력 지수');
		expect(fieldName('record', 'history')).toBe('마일스톤');
	});
});
