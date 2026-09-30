import { describe, expect, test } from 'bun:test';
import { taskDefinitionsOf, vocabularyOf } from '../../../src/lib/task/task-vocabulary';
import {
	taskBusinessColor,
	taskDefinitionBadgeStyle,
	taskTypeColor
} from '../../../src/routes/task/task-definition-colors';

const hexColour = /^#[0-9a-f]{6}$/;

function definitionsOfStoredVocabulary(): ReturnType<typeof taskDefinitionsOf> {
	return taskDefinitionsOf(
		vocabularyOf({
			businesses: [{ name: '오샘플', color: '#2563eb' }, { name: '태스크포스' }],
			types: [{ name: '개발' }, { name: '운영' }, { name: '영업' }]
		})
	);
}

describe('colours a company never chose', () => {
	test('gives every task type its own colour', () => {
		const definitions = definitionsOfStoredVocabulary();
		const colours = definitions.types.map((type) => taskTypeColor(type, definitions));

		expect(new Set(colours).size).toBe(definitions.types.length);
	});

	test('gives colours the badge can actually paint', () => {
		const definitions = definitionsOfStoredVocabulary();

		for (const type of definitions.types) {
			const colour = taskTypeColor(type, definitions);
			expect(colour).toMatch(hexColour);
			expect(taskDefinitionBadgeStyle(colour)).toBe(`background: ${colour}; color: #ffffff;`);
		}
	});

	test('keeps the colour a company did choose', () => {
		const definitions = definitionsOfStoredVocabulary();

		expect(taskBusinessColor('오샘플', definitions)).toBe('#2563eb');
	});

	test('still separates a business nobody coloured from one somebody did', () => {
		const definitions = definitionsOfStoredVocabulary();

		expect(taskBusinessColor('태스크포스', definitions)).toMatch(hexColour);
		expect(taskBusinessColor('태스크포스', definitions)).not.toBe(taskBusinessColor('오샘플', definitions));
	});
});
