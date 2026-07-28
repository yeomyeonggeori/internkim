import { describe, expect, test } from 'bun:test';
import type { OrgGroup } from '../../../src/lib/organization/types';
import { organizationGroupSavePlan } from '../../../src/routes/organization/organization-group-controller';

describe('organization group controller', () => {
	test('reuses an existing organization by normalized name', () => {
		const groups: OrgGroup[] = [{ id: 'product', name: '제품팀' }];

		const plan = organizationGroupSavePlan(groups, ' 제품팀 ', '', () => 'new-group');

		expect(plan).toEqual({
			groupID: 'product',
			groups,
			shouldPersist: false
		});
	});

	test('creates a trimmed organization save plan', () => {
		const groups: OrgGroup[] = [{ id: 'product', name: '제품팀' }];

		const plan = organizationGroupSavePlan(groups, ' 엔지니어링 ', '', () => 'engineering');

		expect(plan).toEqual({
			groupID: 'engineering',
			groups: [
				{ id: 'product', name: '제품팀' },
				{ id: 'engineering', name: '엔지니어링' }
			],
			shouldPersist: true
		});
	});

	test('creates an organization under the selected parent', () => {
		const groups: OrgGroup[] = [{ id: 'product', name: '제품팀' }];

		const plan = organizationGroupSavePlan(groups, ' 개발팀 ', 'product', () => 'engineering');

		expect(plan).toEqual({
			groupID: 'engineering',
			groups: [
				{ id: 'product', name: '제품팀' },
				{ id: 'engineering', name: '개발팀', parentID: 'product' }
			],
			shouldPersist: true
		});
	});
});
