export type UserRecord = {
	userID: string;
	handle: string;
	name?: string;
	email: string;
	image?: string;
	hireDate?: string;
	jobTitle?: string;
	groupID?: string;
	phoneNumber?: string;
	supervisorID?: string;
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
