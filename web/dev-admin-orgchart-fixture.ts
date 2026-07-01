import type { OrgGroup, UserRecord } from './src/routes/admin/admin-types';
import type { DevAdminMockUserRole } from './dev-admin-mock';

export function createDevAdminOrgchartGroups(): OrgGroup[] {
	return [
		{ id: 'group-leadership', name: 'Leadership' },
		{ id: 'group-engineering', name: 'Engineering' },
		{ id: 'group-operations', name: 'Operations' }
	];
}

export function createDevAdminOrgchartUsers(userEmail: string, userRole: DevAdminMockUserRole = 'admin'): UserRecord[] {
	return [
		{
			userID: 'dev-user-ada',
			handle: 'ada',
			name: '김인턴',
			email: userEmail,
			hireDate: '2026-01-03',
			role: userRole,
			jobTitle: 'Founder',
			group: 'group-leadership',
			primaryGroupID: 'group-leadership',
			groupIDs: ['group-leadership'],
			projectIDs: ['blueclaw'],
			teamRole: 'owner',
			employmentStatus: 'active',
			isOrgchartVisible: true,
			status: 'active'
		},
		{
			userID: 'dev-user-grace',
			handle: 'grace',
			name: '이지원',
			email: 'grace@example.com',
			hireDate: '2026-02-10',
			role: 'member',
			jobTitle: 'Engineer',
			group: 'group-engineering',
			primaryGroupID: 'group-engineering',
			groupIDs: ['group-engineering'],
			supervisorID: 'dev-user-ada',
			projectIDs: ['blueclaw', 'admin'],
			teamRole: 'frontend',
			employmentStatus: 'active',
			isOrgchartVisible: true,
			status: 'active'
		},
		{
			userID: 'dev-user-min',
			handle: 'min',
			name: '박민수',
			email: 'min@example.com',
			hireDate: '2026-03-15',
			role: 'member',
			jobTitle: 'Operations Manager',
			group: 'group-operations',
			primaryGroupID: 'group-operations',
			groupIDs: ['group-operations'],
			supervisorID: 'dev-user-ada',
			projectIDs: ['ops'],
			teamRole: 'operations',
			employmentStatus: 'active',
			isOrgchartVisible: true,
			status: 'active'
		}
	];
}
