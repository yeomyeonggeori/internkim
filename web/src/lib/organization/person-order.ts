export type OrganizationSortablePerson = {
	hireDate?: string;
	name?: string;
	email?: string;
};

export type OrganizationHierarchyPerson = OrganizationSortablePerson & {
	userID: string;
	supervisorID?: string;
};

export function orderOrganizationPeopleByHierarchy<T extends OrganizationHierarchyPerson>(people: T[]): T[] {
	const personByUserID = new Map(people.map((person) => [person.userID, person]));
	const directReportsBySupervisorID = new Map<string, T[]>();
	const roots: T[] = [];

	for (const person of people) {
		const supervisorID = person.supervisorID?.trim() ?? '';
		if (!supervisorID || !personByUserID.has(supervisorID)) {
			roots.push(person);
			continue;
		}
		const directReports = directReportsBySupervisorID.get(supervisorID) ?? [];
		directReportsBySupervisorID.set(supervisorID, [...directReports, person]);
	}

	const orderedPeople: T[] = [];
	const visitedUserIDs = new Set<string>();
	const appendPersonAndDirectReports = (person: T): void => {
		if (visitedUserIDs.has(person.userID)) return;
		visitedUserIDs.add(person.userID);
		orderedPeople.push(person);
		const directReports = [...(directReportsBySupervisorID.get(person.userID) ?? [])].sort(compareOrganizationPeople);
		for (const directReport of directReports) appendPersonAndDirectReports(directReport);
	};

	for (const root of [...roots].sort(compareOrganizationPeople)) appendPersonAndDirectReports(root);
	for (const person of [...people].sort(compareOrganizationPeople)) appendPersonAndDirectReports(person);
	return orderedPeople;
}

export function compareOrganizationPeople(first: OrganizationSortablePerson, second: OrganizationSortablePerson): number {
	const hireDateDifference = organizationHireDateSortValue(first).localeCompare(organizationHireDateSortValue(second));
	if (hireDateDifference !== 0) return hireDateDifference;
	return compareOrganizationPersonLabels(organizationPersonLabel(first), organizationPersonLabel(second));
}

function organizationHireDateSortValue(person: OrganizationSortablePerson): string {
	return person.hireDate || '9999-12-31';
}

function organizationPersonLabel(person: OrganizationSortablePerson): string {
	return person.name || person.email || '';
}

function compareOrganizationPersonLabels(first: string, second: string): number {
	const firstRank = organizationPersonLabelScriptRank(first);
	const secondRank = organizationPersonLabelScriptRank(second);
	if (firstRank !== secondRank) return firstRank - secondRank;
	return first.localeCompare(second, firstRank === 1 ? 'en' : 'ko');
}

function organizationPersonLabelScriptRank(label: string): number {
	const trimmedLabel = label.trim();
	if (/^\p{Script=Hangul}/u.test(trimmedLabel)) return 0;
	if (/^[A-Za-z]/.test(trimmedLabel)) return 1;
	return 2;
}
