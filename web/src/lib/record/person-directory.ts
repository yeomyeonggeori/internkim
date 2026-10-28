import { invokeTool } from '$lib/public-api-call';

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

const directoryPageSize = 100;

export async function companyDirectory<Person = RecordPerson>(): Promise<RecordDirectory<Person>> {
	const pages: DirectoryPage<Person>[] = [];
	let cursor: string | undefined;
	do {
		const page = await invokeTool<DirectoryPage<Person>>('person_list', {
			limit: directoryPageSize,
			...(cursor ? { cursor } : {})
		});
		pages.push(page);
		cursor = page.nextCursor;
	} while (cursor);
	const people = pages.flatMap((page) => page.people);
	return { requesterID: pages[0].requesterID, count: people.length, people };
}
