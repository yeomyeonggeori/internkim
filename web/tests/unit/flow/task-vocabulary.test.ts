import { describe, expect, test } from 'bun:test';
import { colourOf, flowDefinitionsOf, vocabularyOf } from '../../../src/lib/flow/task-vocabulary';

describe('colourOf', () => {
	test('keeps the colour someone chose', () => {
		expect(colourOf({ name: '오토케', color: '#2563eb' })).toBe('#2563eb');
	});

	test('gives the same name the same colour every time', () => {
		expect(colourOf({ name: '오토케' })).toBe(colourOf({ name: '오토케' }));
	});

	test('gives different names different colours', () => {
		expect(colourOf({ name: '오토케' })).not.toBe(colourOf({ name: '태스크포스' }));
	});
});

describe('flowDefinitionsOf', () => {
	test('turns the stored vocabulary into what the board reads', () => {
		const definitions = flowDefinitionsOf({
			businesses: [{ name: '오토케', color: '#111111' }],
			types: [{ name: '개발' }],
			sizes: [{ name: 'M', maxHours: 8, score: 3 }]
		});
		expect(definitions.categories).toEqual(['오토케']);
		expect(definitions.categoryColors?.['오토케']).toBe('#111111');
		expect(definitions.types).toEqual(['개발']);
		expect(definitions.typeColors?.['개발']).toMatch(/^hsl\(/);
		expect(definitions.sizes[0]).toMatchObject({ name: 'M', maxHours: 8, score: 3 });
	});

	test('an empty company still produces a usable board', () => {
		const definitions = flowDefinitionsOf(vocabularyOf({}));
		expect(definitions.categories).toEqual([]);
		expect(definitions.sizes).toEqual([]);
	});
});

describe('vocabularyOf', () => {
	test('ignores entries that are not named', () => {
		const vocabulary = vocabularyOf({ businesses: [{ name: '오토케' }, { color: '#fff' }, 'nope'] });
		expect(vocabulary.businesses).toEqual([{ name: '오토케' }]);
	});

	test('survives a column that holds nothing useful', () => {
		expect(vocabularyOf(null)).toEqual({});
		expect(vocabularyOf('{}')).toEqual({});
	});
});
