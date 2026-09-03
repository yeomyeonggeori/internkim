import type { RecordPerson } from '$lib/server/public-api/record/people';

type NamedPerson = Partial<RecordPerson> & { personID: string; name: string; email: string };

export function personInTheDirectory(person: NamedPerson): RecordPerson {
	return {
		isAdmin: false,
		employmentStatus: 'active',
		jobTitle: '',
		teamID: '',
		supervisorID: '',
		phoneNumber: '',
		hireDate: '',
		...person
	};
}
