import { describe, expect, test } from 'bun:test';
import { replaceOrgchartPrimaryGroup } from '../../../src/lib/orgchart/group-membership';

describe('orgchart group membership', () => {
	test('clears every organization membership when primary becomes unassigned', () => {
		const membership = replaceOrgchartPrimaryGroup(
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
		const membership = replaceOrgchartPrimaryGroup(
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
