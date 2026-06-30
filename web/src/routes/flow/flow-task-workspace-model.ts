import type { FlowDefinitions, FlowMember, FlowSummary, FlowTask } from './flow-types';

export const EMPTY_FLOW_BUSINESS_VALUE = '__empty_flow_business__';

export type FlowTaskWorkspaceTab = 'tasks' | 'report' | 'definitions' | 'members';

export type FlowTaskFilterState = {
	searchText: string;
	statusFilter: string;
	participantFilterIDs: string[];
	businessFilter: string;
	typeFilter: string;
};

export type FlowTaskOption = {
	value: string;
	label: string;
	email?: string;
};

const flowTaskListStatusOrder = ['요청', '예정', '진행', '일시정지', '중단', '기각', '완료'];

export function buildFlowTaskTabs(): FlowTaskWorkspaceTab[] {
	return ['tasks', 'report', 'definitions', 'members'];
}

export function defaultParticipantFilterIDs(summary: FlowSummary | null): string[] {
	const member = currentFlowMember(summary);
	return member ? [member.id] : [];
}

export function isDefaultParticipantFilter(participantFilterIDs: string[], summary: FlowSummary | null): boolean {
	const defaultFilterIDs = defaultParticipantFilterIDs(summary);
	return sameStringSet(participantFilterIDs, defaultFilterIDs);
}

export function currentFlowMember(summary: FlowSummary | null): FlowMember | undefined {
	if (!summary?.currentUserEmail) return undefined;
	return summary.members.find((member) => member.email.toLowerCase() === summary.currentUserEmail.toLowerCase());
}

export function filterFlowTasks(tasks: FlowTask[], filters: FlowTaskFilterState): FlowTask[] {
	const normalizedSearch = filters.searchText.trim().toLowerCase();
	return tasks.filter((task) => {
		if (filters.statusFilter !== 'all' && task.status !== filters.statusFilter) return false;
		if (!matchesParticipantFilter(task, filters.participantFilterIDs)) return false;
		if (!matchesBusinessFilter(task, filters.businessFilter)) return false;
		if (filters.typeFilter !== 'all' && task.type !== filters.typeFilter) return false;
		if (!normalizedSearch) return true;
		return taskSearchText(task).includes(normalizedSearch);
	});
}

export function sortFlowTaskList(tasks: FlowTask[]): FlowTask[] {
	return tasks.toSorted(compareFlowTaskListOrder);
}

export function buildMemberFilterOptions(members: FlowMember[], allLabel: string): FlowTaskOption[] {
	return [
		{ value: 'all', label: allLabel },
		...members.map((member) => ({
			value: member.id,
			label: member.name,
			email: member.email
		}))
	];
}

export function buildBusinessFilterOptions(categories: string[], allLabel: string, emptyLabel: string): FlowTaskOption[] {
	return [
		{ value: 'all', label: allLabel },
		{ value: EMPTY_FLOW_BUSINESS_VALUE, label: emptyLabel },
		...categories.map((category) => ({ value: category, label: category }))
	];
}

export function buildBusinessSelectOptions(definitions: FlowDefinitions, emptyLabel: string): FlowTaskOption[] {
	return [
		{ value: EMPTY_FLOW_BUSINESS_VALUE, label: emptyLabel },
		...definitions.categories.map((category) => ({ value: category, label: category }))
	];
}

export function flowBusinessLabel(value: string, emptyLabel: string): string {
	const trimmedValue = value.trim();
	return trimmedValue || emptyLabel;
}

export function flowBusinessOptionValue(value: string): string {
	const trimmedValue = value.trim();
	return trimmedValue || EMPTY_FLOW_BUSINESS_VALUE;
}

export function flowBusinessValueFromOption(value: string): string {
	if (value === EMPTY_FLOW_BUSINESS_VALUE) return '';
	return value;
}

export function canUpdateFlowTask(summary: FlowSummary | null, task: FlowTask): boolean {
	if (summary?.isAdmin) return true;
	const member = currentFlowMember(summary);
	if (!member) return false;
	return member.id === task.ownerID || task.participantIDs.includes(member.id);
}

export function canDeleteFlowTask(summary: FlowSummary | null, task: FlowTask): boolean {
	if (summary?.isAdmin) return true;
	const member = currentFlowMember(summary);
	return member?.id === task.ownerID;
}

export function canManageFlowTaskAssignment(summary: FlowSummary | null, task: FlowTask): boolean {
	if (summary?.isAdmin) return true;
	const member = currentFlowMember(summary);
	return member?.id === task.ownerID;
}

export function canRemoveFlowTaskParticipant(task: FlowTask, memberID: string): boolean {
	return Boolean(memberID) && memberID !== task.ownerID && task.participantIDs.includes(memberID);
}

function matchesParticipantFilter(task: FlowTask, participantFilterIDs: string[]): boolean {
	if (participantFilterIDs.length === 0) return true;
	return participantFilterIDs.some((memberID) => task.participantIDs.includes(memberID));
}

function matchesBusinessFilter(task: FlowTask, businessFilter: string): boolean {
	if (businessFilter === 'all') return true;
	if (businessFilter === EMPTY_FLOW_BUSINESS_VALUE) return task.business.trim() === '';
	return task.business === businessFilter;
}

function taskSearchText(task: FlowTask): string {
	return [
		task.content,
		task.goal,
		task.ownerName,
		task.business,
		task.type,
		...task.participantNames
	].join(' ').toLowerCase();
}

function compareFlowTaskListOrder(left: FlowTask, right: FlowTask): number {
	const statusDifference = statusOrder(left.status) - statusOrder(right.status);
	if (statusDifference !== 0) return statusDifference;

	const dateDifference = compareDescendingDate(taskListSortDate(left), taskListSortDate(right));
	if (dateDifference !== 0) return dateDifference;

	return left.id.localeCompare(right.id);
}

function statusOrder(status: string): number {
	const index = flowTaskListStatusOrder.indexOf(status);
	return index < 0 ? flowTaskListStatusOrder.length : index;
}

function taskListSortDate(task: FlowTask): string {
	return task.endDate?.trim() || task.createdAt?.trim() || '';
}

function compareDescendingDate(left: string, right: string): number {
	if (!left && !right) return 0;
	if (!left) return 1;
	if (!right) return -1;
	return right.localeCompare(left);
}

function sameStringSet(left: string[], right: string[]): boolean {
	if (left.length !== right.length) return false;
	const rightValues = new Set(right);
	return left.every((value) => rightValues.has(value));
}
