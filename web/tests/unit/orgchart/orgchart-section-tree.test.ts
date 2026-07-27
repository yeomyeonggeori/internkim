import { describe, expect, test } from 'bun:test';
import { organizationSectionsWithRecords, organizationSectionTree } from '../../../src/routes/organization/organization-section-tree';
import type { OrganizationOrganizationSection } from '../../../src/routes/organization/organization-model';

function section(id: string, depth: number, records: OrganizationOrganizationSection['records'] = []): OrganizationOrganizationSection {
	return { id, name: id, depth, records, memberCount: records.length };
}

const person = { userID: 'user-1', handle: 'one', email: 'one@example.com' };

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

	test('keeps only the branches that still hold people', () => {
		const tree = organizationSectionTree([
			section('root', 0),
			section('product', 1),
			section('design', 2, [person]),
			section('field', 1)
		]);

		const pruned = organizationSectionsWithRecords(tree);

		expect(pruned.map((node) => node.section.id)).toEqual(['root']);
		expect(pruned[0].children.map((node) => node.section.id)).toEqual(['product']);
		expect(pruned[0].children[0].children.map((node) => node.section.id)).toEqual(['design']);
	});
});
