export type OrgchartSortablePerson = {
	hireDate?: string;
	name?: string;
	email?: string;
};

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
