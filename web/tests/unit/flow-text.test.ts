import { describe, expect, test } from 'bun:test';
import { flowText } from '../../src/routes/flow/text';

type TextTree = { readonly [key: string]: TextNode };
type TextNode = string | readonly string[] | TextTree;

describe('flowText', () => {
	test('keeps Korean and English key shapes aligned', () => {
		expect(collectTextShape(flowText.en).sort()).toEqual(collectTextShape(flowText.ko).sort());
	});

	test('uses business copy for user-facing business labels', () => {
		expect(flowText.ko.report.businessDistance).toBe('이번 주간 사업 거리 분포');
		expect(flowText.ko.definitions.business).toBe('사업');
		expect(flowText.ko.filters.business).toBe('사업');
		expect(flowText.ko.task.business).toBe('사업');
		expect(flowText.ko.table.business).toBe('사업');

		expect(flowText.en.report.businessDistance).toBe('Weekly business distance');
		expect(flowText.en.definitions.business).toBe('Business');
		expect(flowText.en.filters.business).toBe('Business');
		expect(flowText.en.task.business).toBe('Business');
		expect(flowText.en.table.business).toBe('Business');
	});

	test('does not keep user-facing category text keys', () => {
		expect(collectTextShape(flowText.ko).some((path) => path.includes('category'))).toBe(false);
		expect(collectTextShape(flowText.en).some((path) => path.includes('category'))).toBe(false);
	});
});

function collectTextShape(node: TextNode, path: string[] = []): string[] {
	if (typeof node === 'string') return [`${path.join('.')}:string`];
	if (Array.isArray(node)) return [`${path.join('.')}:array:${node.length}`];

	return Object.entries(node).flatMap(([key, value]) => collectTextShape(value, [...path, key]));
}
