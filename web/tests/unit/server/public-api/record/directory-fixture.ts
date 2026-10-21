import { MemberKind } from '$lib/server/public-api/catalog/people';
import type { RecordPerson } from '$lib/server/public-api/record/people';

type NamedPerson = Partial<RecordPerson> & { personID: string; name: string; email: string };

export function personInTheDirectory(person: NamedPerson): RecordPerson {
	return {
		isAdmin: false,
		kind: MemberKind.Human,
		employmentStatus: 'active',
		jobTitle: '',
		teamID: '',
		supervisorID: '',
		phoneNumber: '',
		hireDate: '',
		departureDate: '',
		timeZone: '',
		...person
	};
}
