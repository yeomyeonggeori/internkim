export type UserRecord = {
	memberID: string;
	handle: string;
	name: string;
	email: string;
	image?: string;
	hireDate: string;
	role: 'admin' | 'operationsAdmin' | 'member';
	jobTitle?: string;
	groupID?: string;
	supervisorID?: string;
};

export type UsersResponse = {
	records: UserRecord[];
	availableGroups: { id: string; name: string; parentID?: string }[];
};

export type OrgProfileUpdate = {
	memberID: string;
	email: string;
	jobTitle: string;
	groupID?: string;
	supervisorID?: string;
};

export const initialUsersResponse: UsersResponse = {
	availableGroups: [
		{ id: 'engineering', name: 'Engineering' },
		{ id: 'operations', name: 'Operations' }
	],
	records: [
		{
			memberID: 'user-ada',
			handle: 'ada',
			name: 'Ada Kim',
			email: 'ada@example.com',
			hireDate: '2026-01-02',
			role: 'admin',
			jobTitle: 'Founder',
			groupID: 'engineering'
		},
		{
			memberID: 'user-grace',
			handle: 'grace',
			name: 'Grace Lee',
			email: 'grace@example.com',
			hireDate: '2026-02-03',
			role: 'member',
			jobTitle: '',
			groupID: ''
		}
	]
};
