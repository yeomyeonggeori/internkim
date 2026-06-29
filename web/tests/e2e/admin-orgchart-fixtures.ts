export type UserRecord = {
	userID: string;
	handle: string;
	name: string;
	email: string;
	hireDate: string;
	role: 'admin' | 'member';
	jobTitle?: string;
	group?: string;
	positionLevel?: number;
	primaryGroupID?: string;
	groupIDs?: string[];
	supervisorID?: string;
	projectIDs?: string[];
	teamRole?: string;
	employmentStatus?: 'active' | 'leave' | 'resigned';
	isOrgchartVisible?: boolean;
};

export type UsersResponse = {
	records: UserRecord[];
	availableGroups: { id: string; name: string }[];
};

export type OrgProfileUpdate = {
	userID: string;
	email: string;
	jobTitle: string;
	group?: string;
	positionLevel?: number;
	primaryGroupID?: string;
	groupIDs?: string[];
	supervisorID?: string;
	projectIDs?: string[];
	teamRole?: string;
	employmentStatus?: 'active' | 'leave' | 'resigned';
	isOrgchartVisible?: boolean;
};

export const initialUsersResponse: UsersResponse = {
	availableGroups: [
		{ id: 'engineering', name: 'Engineering' },
		{ id: 'operations', name: 'Operations' }
	],
	records: [
		{
			userID: 'user-ada',
			handle: 'ada',
			name: 'Ada Kim',
			email: 'ada@example.com',
			hireDate: '2026-01-02',
			role: 'admin',
			jobTitle: 'Founder',
			primaryGroupID: 'engineering',
			groupIDs: ['engineering'],
			employmentStatus: 'active',
			isOrgchartVisible: true
		},
		{
			userID: 'user-grace',
			handle: 'grace',
			name: 'Grace Lee',
			email: 'grace@example.com',
			hireDate: '2026-02-03',
			role: 'member',
			jobTitle: '',
			primaryGroupID: '',
			groupIDs: [],
			projectIDs: [],
			employmentStatus: 'active',
			isOrgchartVisible: true
		}
	]
};
