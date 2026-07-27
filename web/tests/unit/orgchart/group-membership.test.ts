import { describe, expect, test } from 'bun:test';
import { replaceOrganizationPrimaryGroup } from '../../../src/lib/organization/group-membership';

describe('organization group membership', () => {
	test('clears every organization membership when primary becomes unassigned', () => {
		const membership = replaceOrganizationPrimaryGroup(
			{
				primaryGroupID: 'product',
				group: 'product',
				groupIDs: ['product', 'platform']
			},
			''
		);

		expect(membership).toEqual({ primaryGroupID: '', groupIDs: [] });
	});

	test('replaces only the previous primary and preserves secondary organizations', () => {
		const membership = replaceOrganizationPrimaryGroup(
			{
				primaryGroupID: 'product',
				group: 'product',
				groupIDs: [' product ', 'platform', 'platform']
			},
			' operations '
		);

		expect(membership).toEqual({ primaryGroupID: 'operations', groupIDs: ['operations', 'platform'] });
	});
});
