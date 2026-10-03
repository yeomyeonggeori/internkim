import { describe, expect, test } from 'bun:test';
import { readFileSync } from 'node:fs';
import { dataRoomTemplate, filingCategories, normalizeCategoryGrants } from '../../../src/lib/data-room/model';

describe('the company data room template', () => {
	test('every circle names existing mnemonic scopes without redundant descendants', () => {
		const { categories, circles } = dataRoomTemplate;
		expect(categories.filter((category) => category.parent !== null)).toHaveLength(37);
		expect(categories.filter((category) => category.parent === null)).toHaveLength(11);
		expect(new Set(categories.map((category) => category.code)).size).toBe(categories.length);
		for (const category of categories.filter((category) => category.parent !== null)) {
			expect(category.parent).toBe(category.code[0]);
		}
		for (const circle of circles) {
			expect(normalizeCategoryGrants(circle.readableCategories, categories)).toEqual(circle.readableCategories);
		}
	});

	test('filing choices include the inbox and exclude parents with children', () => {
		const choices = filingCategories(dataRoomTemplate.categories);
		expect(choices).toHaveLength(38);
		expect(choices.some((category) => category.code === 'X')).toBe(true);
		expect(choices.some((category) => category.code === 'F')).toBe(false);
	});

	test('unknown codes are refused instead of silently removed', () => {
		expect(normalizeCategoryGrants(['F', 'FS', 'F'], dataRoomTemplate.categories)).toEqual(['F']);
		expect(normalizeCategoryGrants(['missing'], dataRoomTemplate.categories)).toBeUndefined();
	});

	test('a parent is a filing destination only while it has no children', () => {
		const categories = dataRoomTemplate.categories;
		const withoutFinanceChildren = categories.filter((category) => category.parent !== 'F');
		expect(filingCategories(withoutFinanceChildren).some((category) => category.code === 'F')).toBe(true);
		expect(filingCategories(categories).some((category) => category.code === 'F')).toBe(false);
	});

	test('database initialization and the portable skill derive from the same source', () => {
		const migration = readFileSync(new URL('../../../../supabase/migrations/20261004000006_the_data_room_template_names_circles.sql', import.meta.url), 'utf8');
		const embedded = migration.split('$json$')[1];
		expect(JSON.parse(embedded ?? '')).toEqual(dataRoomTemplate);
		const portable = readFileSync(new URL('../../../../.dependency/internkim-plugin/skills/dataroom/assets/template.json', import.meta.url), 'utf8');
		expect(JSON.parse(portable)).toEqual(dataRoomTemplate);
	});
});
