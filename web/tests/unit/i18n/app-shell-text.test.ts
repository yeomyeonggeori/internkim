import { describe, expect, test } from 'bun:test';

import { appShellText } from '../../../src/lib/i18n/app-shell-text';

type TextTree = { readonly [key: string]: TextNode };
type TextNode = string | readonly string[] | TextTree;

describe('appShellText', () => {
	test('keeps Korean and English key shapes aligned', () => {
		expect(collectTextShape(appShellText.en).sort()).toEqual(collectTextShape(appShellText.ko).sort());
	});
});

function collectTextShape(node: TextNode, path: string[] = []): string[] {
	if (typeof node === 'string') return [`${path.join('.')}:string`];
	if (Array.isArray(node)) return [`${path.join('.')}:array:${node.length}`];

	return Object.entries(node).flatMap(([key, value]) => collectTextShape(value, [...path, key]));
}
