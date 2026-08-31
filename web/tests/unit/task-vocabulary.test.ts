import { describe, expect, test } from 'bun:test';
import { taskDefinitionsOf, taskVocabularyOfDefinitions, vocabularyOf } from '../../src/lib/task/task-vocabulary';

describe('task vocabulary', () => {
	test('round-trips names, colors, and the etc colors', () => {
		const vocabulary = vocabularyOf({
			businesses: [{ name: '사업하나', color: '#216fe4' }, { name: '사업둘' }],
			types: [{ name: '기능' }],
			etcBusinessColor: '#DC2626',
			etcTypeColor: '#0891b2'
		});
		const definitions = taskDefinitionsOf(vocabulary);

		expect(definitions.categories).toEqual(['사업하나', '사업둘']);
		expect(definitions.categoryColors).toEqual({ 사업하나: '#216fe4' });
		expect(definitions.etcBusinessColor).toBe('#DC2626');
		expect(definitions.etcTypeColor).toBe('#0891b2');
		expect(taskVocabularyOfDefinitions(definitions)).toEqual({
			businesses: [{ name: '사업하나', color: '#216fe4' }, { name: '사업둘' }],
			types: [{ name: '기능' }],
			etcBusinessColor: '#DC2626',
			etcTypeColor: '#0891b2'
		});
	});

	test('drops blank etc colors instead of storing them', () => {
		const definitions = taskDefinitionsOf(vocabularyOf({ etcBusinessColor: ' ' }));

		expect(definitions.etcBusinessColor).toBeUndefined();
		expect(taskVocabularyOfDefinitions(definitions)).toEqual({ businesses: [], types: [] });
	});
});
