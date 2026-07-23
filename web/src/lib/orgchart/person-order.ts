export type OrgchartSortablePerson = {
	hireDate?: string;
	name?: string;
	email?: string;
};

export type OrgchartHierarchyPerson = OrgchartSortablePerson & {
	userID: string;
	supervisorID?: string;
};

export function orderOrgchartPeopleByHierarchy<T extends OrgchartHierarchyPerson>(people: T[]): T[] {
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
		const directReports = [...(directReportsBySupervisorID.get(person.userID) ?? [])].sort(compareOrgchartPeople);
		for (const directReport of directReports) appendPersonAndDirectReports(directReport);
	};

	for (const root of [...roots].sort(compareOrgchartPeople)) appendPersonAndDirectReports(root);
	for (const person of [...people].sort(compareOrgchartPeople)) appendPersonAndDirectReports(person);
	return orderedPeople;
}

export function compareOrgchartPeople(first: OrgchartSortablePerson, second: OrgchartSortablePerson): number {
	const hireDateDifference = orgchartHireDateSortValue(first).localeCompare(orgchartHireDateSortValue(second));
	if (hireDateDifference !== 0) return hireDateDifference;
	return compareOrgchartPersonLabels(orgchartPersonLabel(first), orgchartPersonLabel(second));
}

function orgchartHireDateSortValue(person: OrgchartSortablePerson): string {
	return person.hireDate || '9999-12-31';
}

function orgchartPersonLabel(person: OrgchartSortablePerson): string {
	return person.name || person.email || '';
}

function compareOrgchartPersonLabels(first: string, second: string): number {
	const firstRank = orgchartPersonLabelScriptRank(first);
	const secondRank = orgchartPersonLabelScriptRank(second);
	if (firstRank !== secondRank) return firstRank - secondRank;
	return first.localeCompare(second, firstRank === 1 ? 'en' : 'ko');
}

function orgchartPersonLabelScriptRank(label: string): number {
	const trimmedLabel = label.trim();
	if (/^\p{Script=Hangul}/u.test(trimmedLabel)) return 0;
	if (/^[A-Za-z]/.test(trimmedLabel)) return 1;
	return 2;
}
