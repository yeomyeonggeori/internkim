import { everyPage } from './every-page';

export type RecordPerson = {
	personID: string;
	name: string;
	email: string;
	isAdmin?: boolean;
	hireDate?: string;
	timeZone?: string;
};

export type RecordDirectory<Person = RecordPerson> = {
	requesterID: string;
	count: number;
	people: Person[];
};

type DirectoryPage<Person> = RecordDirectory<Person> & { nextCursor?: string };

export async function companyDirectory<Person = RecordPerson>(): Promise<RecordDirectory<Person>> {
	const pages = await everyPage<DirectoryPage<Person>>('person_list', {});
	const people = pages.flatMap((page) => page.people);
	return { requesterID: pages[0].requesterID, count: people.length, people };
}
