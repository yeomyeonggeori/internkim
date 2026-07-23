import type { OrgGroup } from './src/routes/admin/admin-types';

export function createDevAdminOrgchartGroups(): OrgGroup[] {
	return [
		{ id: 'group-leadership', name: '경영' },
		{ id: 'group-operations', name: '운영팀' },
		{ id: 'group-product', name: '제품팀' },
		{ id: 'group-design', name: '디자인팀', parentID: 'group-product' },
		{ id: 'group-field', name: '현장지원팀' }
	];
}
