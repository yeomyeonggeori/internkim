import { describe, expect, test } from 'bun:test';
import { flowBusinessColor, flowSizeColor, flowTaskTypeColor } from '../../src/routes/flow/flow-definition-colors';
import { flowProjectColor, flowTypeColor } from '../../src/routes/flow/flow-report-colors';
import type { FlowDefinitions } from '../../src/routes/flow/flow-types';

describe('flow definition colors', () => {
	test('prefers the color saved in the definitions', () => {
		const definitions = flowDefinitions({
			categories: ['여명거리', '김인턴'],
			categoryColors: { 김인턴: '#DC2626' },
			types: ['기능'],
			typeColors: { 기능: '#0891b2' }
		});

		expect(flowBusinessColor('김인턴', definitions)).toBe('#dc2626');
		expect(flowTaskTypeColor('기능', definitions)).toBe('#0891b2');
	});

	test('falls back to the definitions order palette when no color is saved', () => {
		const definitions = flowDefinitions({ categories: ['여명거리', '김인턴'], types: ['기획', '기능'] });

		expect(flowBusinessColor('여명거리', definitions)).toBe(flowProjectColor(0));
		expect(flowTaskTypeColor('기능', definitions)).toBe(flowTypeColor(1));
	});

	test('falls back to a neutral color for values the definitions do not list', () => {
		expect(flowBusinessColor('사라진 사업', flowDefinitions({}))).toBe('#64748b');
	});

	test('reports no size color until one is saved', () => {
		expect(flowSizeColor('M', flowDefinitions({}))).toBe('');
		expect(flowSizeColor('M', flowDefinitions({ sizes: [size('M', '#2563EB')] }))).toBe('#2563eb');
	});
});

function flowDefinitions(overrides: Partial<FlowDefinitions>): FlowDefinitions {
	return { categories: [], types: [], sizes: [], ...overrides };
}

function size(name: string, color: string) {
	return { name, distanceKm: 3, maxHours: 8, developmentExample: '', otherExample: '', note: '', color, score: 3, label: '' };
}
