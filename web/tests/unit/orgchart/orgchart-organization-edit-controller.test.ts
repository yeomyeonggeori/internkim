import { describe, expect, test } from 'bun:test';
import type { OrgGroup } from '../../../src/lib/orgchart/types';

Object.assign(globalThis, {
	$state<Value>(value: Value): Value {
		return value;
	}
});

const { OrgchartOrganizationEditController } = await import('../../../src/routes/orgchart/orgchart-organization-edit-controller.svelte');

describe('orgchart organization edit controller', () => {
	test('keeps hierarchy moves in a draft until editing ends', () => {
		const groups: OrgGroup[] = [
			{ id: 'product', name: '프로덕트 본부' },
			{ id: 'engineering', name: '개발팀', parentID: 'product' },
			{ id: 'sales', name: '세일즈' }
		];
		const controller = new OrgchartOrganizationEditController();

		controller.begin(groups);
		controller.move('engineering', 2, 0);

		expect(controller.isEditing).toBe(true);
		expect(controller.draftGroups).toEqual([
			{ id: 'product', name: '프로덕트 본부' },
			{ id: 'sales', name: '세일즈' },
			{ id: 'engineering', name: '개발팀', parentID: '' }
		]);
		expect(groups[1]?.parentID).toBe('product');
	});

	test('cancels the hierarchy draft', () => {
		const controller = new OrgchartOrganizationEditController();
		controller.begin([{ id: 'product', name: '프로덕트 본부' }]);

		controller.cancel();

		expect(controller.isEditing).toBe(false);
		expect(controller.draftGroups).toEqual([]);
	});
});
