import type { UserRole } from '$lib/types';

export type UserRecord = {
	memberID: string;
	handle: string;
	name?: string;
	email: string;
	image?: string;
	hireDate?: string;
	note?: string;
	role?: UserRole;
	circles?: string[];
	jobTitle?: string;
	groupID?: string;
	phoneNumber?: string;
	supervisorID?: string;
	status?: string;
	isIncomplete?: boolean;
};

export type OrgGroup = {
	id: string;
	name: string;
	parentID?: string;
};

export type UsersResponse = {
	records?: UserRecord[];
	availableGroups?: OrgGroup[];
};

export type OrgProfileUpdate = {
	memberID: string;
	jobTitle: string;
	groupID?: string;
	hireDate?: string;
	phoneNumber?: string;
	supervisorID?: string;
};
