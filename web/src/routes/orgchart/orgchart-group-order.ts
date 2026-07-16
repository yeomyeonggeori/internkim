import type { Locale } from '../../lib/i18n/locale.svelte';
import { orgchartGroupMembership } from '../../lib/orgchart/group-membership';
import type { UserRecord } from '../../lib/orgchart/types';

export type OrgchartGroupOrderItem = {
	id: string;
	name: string;
	isUnassigned: boolean;
};

const highestExecutiveTitles = new Set(['ceo', 'founder']);
const secondExecutiveTitles = new Set(['coceo', 'cofounder']);
const cLevelTitles = new Set([
	'cao',
	'caio',
	'cbo',
	'cco',
	'cdao',
	'cdo',
	'cfo',
	'cgo',
	'chro',
	'cio',
	'ciso',
	'cko',
	'clo',
	'cmo',
	'cno',
	'coo',
	'cpo',
	'cro',
	'cso',
	'cto',
	'cvo',
	'cwo',
	'cxo'
]);
const generalGroupPriority = 3;

export function compareOrgchartGroups(records: UserRecord[], locale: Locale): (first: OrgchartGroupOrderItem, second: OrgchartGroupOrderItem) => number {
	const priorities = orgchartGroupPriorities(records);
	return (first, second) => {
		if (first.isUnassigned !== second.isUnassigned) return first.isUnassigned ? 1 : -1;
		const priorityDifference = (priorities.get(first.id) ?? generalGroupPriority) - (priorities.get(second.id) ?? generalGroupPriority);
		if (priorityDifference !== 0) return priorityDifference;
		return first.name.localeCompare(second.name, locale) || first.id.localeCompare(second.id, locale);
	};
}

function orgchartGroupPriorities(records: UserRecord[]): Map<string, number> {
	const priorities = new Map<string, number>();
	for (const record of records) {
		const priority = orgchartGroupPriority(record.jobTitle);
		for (const groupID of orgchartGroupMembership(record).groupIDs) {
			priorities.set(groupID, Math.min(priorities.get(groupID) ?? generalGroupPriority, priority));
		}
	}
	return priorities;
}

function orgchartGroupPriority(jobTitle: string | undefined): number {
	const normalizedJobTitle = (jobTitle ?? '').toLowerCase().replace(/[\s-]+/g, '');
	if (highestExecutiveTitles.has(normalizedJobTitle)) return 0;
	if (secondExecutiveTitles.has(normalizedJobTitle)) return 1;
	if (cLevelTitles.has(normalizedJobTitle)) return 2;
	return generalGroupPriority;
}
