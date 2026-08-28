import { describe, expect, test } from 'bun:test';
import { colourOf, taskDefinitionsOf, taskVocabularyOfDefinitions, vocabularyOf } from '../../../src/lib/task/task-vocabulary';

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

describe('taskDefinitionsOf', () => {
	test('turns the stored vocabulary into what the board reads', () => {
		const definitions = taskDefinitionsOf({
			businesses: [{ name: '오토케', color: '#111111' }],
			types: [{ name: '개발' }]
		});
		expect(definitions.categories).toEqual(['오토케']);
		expect(definitions.categoryColors?.['오토케']).toBe('#111111');
		expect(definitions.types).toEqual(['개발']);
		expect(definitions.sizes.map((size) => size.name)).toEqual(['XS', 'S', 'M', 'L', 'XL', 'XXL']);
	});

	test('leaves a value nobody coloured out of the stored colours', () => {
		const definitions = taskDefinitionsOf({ types: [{ name: '개발' }, { name: '운영', color: '#222222' }] });
		expect(definitions.typeColors?.['개발']).toBe(undefined);
		expect(definitions.typeColors?.['운영']).toBe('#222222');
	});

	test('sizes are the same for a company that has chosen nothing', () => {
		const definitions = taskDefinitionsOf(vocabularyOf({}));
		expect(definitions.categories).toEqual([]);
		expect(definitions.sizes.map((size) => size.name)).toEqual(['XS', 'S', 'M', 'L', 'XL', 'XXL']);
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

describe('taskVocabularyOfDefinitions', () => {
	test('turns editor definitions back into the stored vocabulary', () => {
		const vocabulary = taskVocabularyOfDefinitions({
			categories: ['오토케', '태스크포스'],
			categoryColors: { 오토케: '#111111' },
			types: ['개발'],
			typeColors: {},
			sizes: []
		});
		expect(vocabulary.businesses).toEqual([{ name: '오토케', color: '#111111' }, { name: '태스크포스' }]);
		expect(vocabulary.types).toEqual([{ name: '개발' }]);
	});

	test('round-trips through the definitions the board reads', () => {
		const stored = {
			businesses: [{ name: '오토케', color: '#111111' }, { name: '태스크포스' }],
			types: [{ name: '개발' }, { name: '운영', color: '#222222' }]
		};
		expect(taskVocabularyOfDefinitions(taskDefinitionsOf(stored))).toEqual(stored);
	});
});
