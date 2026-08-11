import { describe, expect, test } from 'bun:test';
import { flowDefinitionsOf, vocabularyOf } from '../../../src/lib/flow/task-vocabulary';
import {
	flowBusinessColor,
	flowDefinitionBadgeStyle,
	flowTaskTypeColor
} from '../../../src/routes/flow/flow-definition-colors';

const hexColour = /^#[0-9a-f]{6}$/;

function definitionsOfStoredVocabulary(): ReturnType<typeof flowDefinitionsOf> {
	return flowDefinitionsOf(
		vocabularyOf({
			businesses: [{ name: '오토케', color: '#2563eb' }, { name: '태스크포스' }],
			types: [{ name: '개발' }, { name: '운영' }, { name: '영업' }]
		})
	);
}

describe('colours a company never chose', () => {
	test('gives every task type its own colour', () => {
		const definitions = definitionsOfStoredVocabulary();
		const colours = definitions.types.map((type) => flowTaskTypeColor(type, definitions));

		expect(new Set(colours).size).toBe(definitions.types.length);
	});

	test('gives colours the badge can actually paint', () => {
		const definitions = definitionsOfStoredVocabulary();

		for (const type of definitions.types) {
			const colour = flowTaskTypeColor(type, definitions);
			expect(colour).toMatch(hexColour);
			expect(flowDefinitionBadgeStyle(colour)).toBe(`background: ${colour}; color: #ffffff;`);
		}
	});

	test('keeps the colour a company did choose', () => {
		const definitions = definitionsOfStoredVocabulary();

		expect(flowBusinessColor('오토케', definitions)).toBe('#2563eb');
	});

	test('still separates a business nobody coloured from one somebody did', () => {
		const definitions = definitionsOfStoredVocabulary();

		expect(flowBusinessColor('태스크포스', definitions)).toMatch(hexColour);
		expect(flowBusinessColor('태스크포스', definitions)).not.toBe(flowBusinessColor('오토케', definitions));
	});
});
