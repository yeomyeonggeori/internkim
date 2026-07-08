export type OrgchartSortablePerson = {
	hireDate?: string;
	name?: string;
	email?: string;
};

export function compareOrgchartPeople(first: OrgchartSortablePerson, second: OrgchartSortablePerson): number {
	const hireDateDifference = orgchartHireDateSortValue(first).localeCompare(orgchartHireDateSortValue(second));
	if (hireDateDifference !== 0) return hireDateDifference;
	return orgchartPersonLabel(first).localeCompare(orgchartPersonLabel(second));
}

function orgchartHireDateSortValue(person: OrgchartSortablePerson): string {
	return person.hireDate || '9999-12-31';
}

function orgchartPersonLabel(person: OrgchartSortablePerson): string {
	return person.name || person.email || '';
}
