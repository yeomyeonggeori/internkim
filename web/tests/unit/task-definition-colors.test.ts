import { describe, expect, test } from 'bun:test';
import { taskBusinessColor, taskDefinitionPaletteColor, taskTypeColor } from '../../src/routes/task/task-definition-colors';
import type { TaskDefinitions } from '../../src/routes/task/task-types';

describe('flow definition colors', () => {
	test('prefers the color saved in the definitions', () => {
		const definitions = taskDefinitions({
			categories: ['샘플거리', '김인턴'],
			categoryColors: { 김인턴: '#DC2626' },
			types: ['기능'],
			typeColors: { 기능: '#0891b2' }
		});

		expect(taskBusinessColor('김인턴', definitions)).toBe('#dc2626');
		expect(taskTypeColor('기능', definitions)).toBe('#0891b2');
	});

	test('falls back to the definitions order palette when no color is saved', () => {
		const definitions = taskDefinitions({ categories: ['샘플거리', '김인턴'], types: ['기획', '기능'] });

		expect(taskBusinessColor('샘플거리', definitions)).toBe(taskDefinitionPaletteColor(0));
		expect(taskTypeColor('기능', definitions)).toBe(taskDefinitionPaletteColor(1));
	});

	test('mixes default colors so neighbouring definitions look different', () => {
		const defaults = [0, 1, 2, 3, 4, 5].map((index) => taskDefinitionPaletteColor(index));

		expect(new Set(defaults).size).toBe(defaults.length);
		expect(defaults.some((color, index) => index > 0 && color === defaults[index - 1])).toBe(false);
		expect(taskDefinitionPaletteColor(0)).not.toBe(taskDefinitionPaletteColor(1));
	});

	test('falls back to a neutral color for values the definitions do not list', () => {
		expect(taskBusinessColor('사라진 사업', taskDefinitions({}))).toBe('#64748b');
	});

	test('colors a null value with the saved etc color', () => {
		const definitions = taskDefinitions({ etcBusinessColor: '#DC2626', etcTypeColor: '#0891b2' });

		expect(taskBusinessColor(null, definitions)).toBe('#DC2626');
		expect(taskTypeColor(null, definitions)).toBe('#0891b2');
		expect(taskBusinessColor(null, taskDefinitions({}))).toBe('#64748b');
		expect(taskTypeColor(null, taskDefinitions({}))).toBe('#64748b');
	});
});

function taskDefinitions(overrides: Partial<TaskDefinitions>): TaskDefinitions {
	return { categories: [], types: [], sizes: [], ...overrides };
}
