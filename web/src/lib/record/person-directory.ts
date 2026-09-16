import { invokeTool } from '$lib/public-api-call';

export type RecordPerson = {
	personID: string;
	name: string;
	email: string;
	isAdmin?: boolean;
	clearance?: number;
	hireDate?: string;
	timeZone?: string;
};

export type RecordDirectory = {
	requesterID: string;
	count: number;
	people: RecordPerson[];
};

export function companyDirectory(): Promise<RecordDirectory> {
	return invokeTool('person_list', {});
}
