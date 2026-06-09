import { describe, expect, test } from 'bun:test';
import { flowText } from '../../src/routes/flow/text';

type TextTree = { readonly [key: string]: TextNode };
type TextNode = string | readonly string[] | TextTree;

describe('flowText', () => {
	test('keeps Korean and English key shapes aligned', () => {
		expect(collectTextShape(flowText.en).sort()).toEqual(collectTextShape(flowText.ko).sort());
	});

	test('uses category copy for user-facing category labels', () => {
		expect(flowText.ko.report.businessDistance).toBe('이번 주간 대분류 거리 분포');
		expect(flowText.ko.definitions.category).toBe('대분류');
		expect(flowText.ko.filters.business).toBe('대분류');
		expect(flowText.ko.task.category).toBe('대분류');
		expect(flowText.ko.table.business).toBe('대분류');

		expect(flowText.en.report.businessDistance).toBe('Weekly category distance');
		expect(flowText.en.definitions.category).toBe('Category');
		expect(flowText.en.filters.business).toBe('Category');
		expect(flowText.en.task.category).toBe('Category');
		expect(flowText.en.table.business).toBe('Category');
	});
});

function collectTextShape(node: TextNode, path: string[] = []): string[] {
	if (typeof node === 'string') return [`${path.join('.')}:string`];
	if (Array.isArray(node)) return [`${path.join('.')}:array:${node.length}`];

	return Object.entries(node).flatMap(([key, value]) => collectTextShape(value, [...path, key]));
}
