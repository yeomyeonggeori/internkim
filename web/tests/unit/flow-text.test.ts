import { describe, expect, test } from 'bun:test';
import { flowText } from '../../src/routes/flow/text';

type TextTree = { readonly [key: string]: TextNode };
type TextNode = string | readonly string[] | TextTree;

describe('flowText', () => {
	test('keeps Korean and English key shapes aligned', () => {
		expect(collectTextShape(flowText.en).sort()).toEqual(collectTextShape(flowText.ko).sort());
	});
});

function collectTextShape(node: TextNode, path: string[] = []): string[] {
	if (typeof node === 'string') return [`${path.join('.')}:string`];
	if (Array.isArray(node)) return [`${path.join('.')}:array:${node.length}`];

	return Object.entries(node).flatMap(([key, value]) => collectTextShape(value, [...path, key]));
}
