import { describe, expect, test } from 'bun:test';
import { organizationDirectoryText } from '../../../src/routes/organization/text';

type TextTree = { readonly [key: string]: TextNode };
type TextNode = string | readonly string[] | TextTree;

describe('organization directory text', () => {
	test('keeps Korean and English key shapes aligned', () => {
		expect(collectTextShape(organizationDirectoryText.en).sort()).toEqual(collectTextShape(organizationDirectoryText.ko).sort());
	});

	test('names the directory and synthetic root organization explicitly', () => {
		expect(organizationDirectoryText.ko.title).toBe('조직');
		expect(organizationDirectoryText.ko.allOrganizations).toBe('전체');
	});
});

function collectTextShape(node: TextNode, path: string[] = []): string[] {
	if (typeof node === 'string') return [`${path.join('.')}:string`];
	if (Array.isArray(node)) return [`${path.join('.')}:array:${node.length}`];

	return Object.entries(node).flatMap(([key, value]) => collectTextShape(value, [...path, key]));
}
