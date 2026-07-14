import { describe, expect, test } from 'bun:test';
import type { OrgGroup } from '../../../src/lib/orgchart/types';
import { orgchartGroupSavePlan } from '../../../src/routes/orgchart/orgchart-group-controller';

describe('orgchart group controller', () => {
	test('reuses an existing organization by normalized name', () => {
		const groups: OrgGroup[] = [{ id: 'product', name: '제품팀' }];

		const plan = orgchartGroupSavePlan(groups, ' 제품팀 ', () => 'new-group');

		expect(plan).toEqual({
			groupID: 'product',
			groups,
			shouldPersist: false
		});
	});

	test('creates a trimmed organization save plan', () => {
		const groups: OrgGroup[] = [{ id: 'product', name: '제품팀' }];

		const plan = orgchartGroupSavePlan(groups, ' 엔지니어링 ', () => 'engineering');

		expect(plan).toEqual({
			groupID: 'engineering',
			groups: [
				{ id: 'product', name: '제품팀' },
				{ id: 'engineering', name: '엔지니어링' }
			],
			shouldPersist: true
		});
	});
});
