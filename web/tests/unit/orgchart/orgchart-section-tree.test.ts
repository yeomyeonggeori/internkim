import { describe, expect, test } from 'bun:test';
import { organizationSectionTree } from '../../../src/routes/organization/organization-section-tree';
import type { OrganizationOrganizationSection } from '../../../src/routes/organization/organization-model';

function section(id: string, depth: number): OrganizationOrganizationSection {
	return { id, name: id, depth, records: [], memberCount: 0 };
}

describe('organization section tree', () => {
	test('nests sections under the closest shallower section', () => {
		const tree = organizationSectionTree([section('root', 0), section('product', 1), section('design', 2), section('field', 1)]);

		expect(tree.map((node) => node.section.id)).toEqual(['root']);
		expect(tree[0].children.map((node) => node.section.id)).toEqual(['product', 'field']);
		expect(tree[0].children[0].children.map((node) => node.section.id)).toEqual(['design']);
	});

	test('keeps sibling roots side by side', () => {
		const tree = organizationSectionTree([section('product', 0), section('field', 0)]);

		expect(tree.map((node) => node.section.id)).toEqual(['product', 'field']);
	});
});
