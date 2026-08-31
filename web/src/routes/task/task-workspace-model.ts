import type { TaskDefinitions, TaskMember, TaskSummary, Task } from './task-types';

export const ETC_TASK_OPTION_VALUE = '__etc_task_option__';

export function normalizedTaskDefinitionValue(value: string | null | undefined): string | null {
	const trimmedValue = value?.trim();
	return trimmedValue ? trimmedValue : null;
}

export type TaskWorkspaceTab = 'tasks' | 'report' | 'definitions' | 'members';

export type TaskFilterState = {
	searchText: string;
	statusFilter: string;
	participantFilterIDs: string[];
	businessFilter: string;
	typeFilter: string;
};

export type TaskOption = {
	value: string;
	label: string;
	email?: string;
	image?: string;
};

const taskListStatusOrder = ['requested', 'planned', 'in_progress', 'paused', 'stopped', 'rejected', 'completed'];

export function buildTaskTabs(): TaskWorkspaceTab[] {
	return ['tasks', 'report', 'definitions', 'members'];
}

export function defaultParticipantFilterIDs(summary: TaskSummary | null): string[] {
	const member = currentTaskMember(summary);
	return member ? [member.id] : [];
}

export function isDefaultParticipantFilter(participantFilterIDs: string[], summary: TaskSummary | null): boolean {
	const defaultFilterIDs = defaultParticipantFilterIDs(summary);
	return sameStringSet(participantFilterIDs, defaultFilterIDs);
}

export function currentTaskMember(
	summary: { members: TaskMember[]; currentUserEmail: string } | null
): TaskMember | undefined {
	if (!summary?.currentUserEmail) return undefined;
	return summary.members.find((member) => member.email.toLowerCase() === summary.currentUserEmail.toLowerCase());
}

export function filterTasks(tasks: Task[], filters: TaskFilterState): Task[] {
	const normalizedSearch = filters.searchText.trim().toLowerCase();
	return tasks.filter((task) => {
		if (filters.statusFilter !== 'all' && task.status !== filters.statusFilter) return false;
		if (!matchesParticipantFilter(task, filters.participantFilterIDs)) return false;
		if (!matchesDefinitionFilter(task.business, filters.businessFilter)) return false;
		if (!matchesDefinitionFilter(task.type, filters.typeFilter)) return false;
		if (!normalizedSearch) return true;
		return taskSearchText(task).includes(normalizedSearch);
	});
}

export function sortTaskList(tasks: Task[]): Task[] {
	return tasks.toSorted(compareTaskListOrder);
}

export function buildMemberFilterOptions(members: TaskMember[], allLabel: string): TaskOption[] {
	return [
		{ value: 'all', label: allLabel },
		...members.map((member) => ({
			value: member.id,
			label: member.name,
			email: member.email,
			image: member.image
		}))
	];
}

export function buildBusinessFilterOptions(categories: string[], allLabel: string, etcLabel: string): TaskOption[] {
	return [
		{ value: 'all', label: allLabel },
		{ value: ETC_TASK_OPTION_VALUE, label: etcLabel },
		...categories.map((category) => ({ value: category, label: category }))
	];
}

export function buildBusinessSelectOptions(definitions: TaskDefinitions, etcLabel: string): TaskOption[] {
	return [
		{ value: ETC_TASK_OPTION_VALUE, label: etcLabel },
		...definitions.categories.map((category) => ({ value: category, label: category }))
	];
}

export function buildTypeSelectOptions(definitions: TaskDefinitions, etcLabel: string): TaskOption[] {
	return [
		{ value: ETC_TASK_OPTION_VALUE, label: etcLabel },
		...definitions.types.map((type) => ({ value: type, label: type }))
	];
}

export function taskDefinitionLabel(value: string | null, etcLabel: string): string {
	return value ?? etcLabel;
}

export function taskDefinitionOptionValue(value: string | null): string {
	return value ?? ETC_TASK_OPTION_VALUE;
}

export function taskDefinitionValueFromOption(value: string): string | null {
	if (value === ETC_TASK_OPTION_VALUE) return null;
	return value;
}

export function canUpdateTask(summary: TaskSummary | null, task: Task): boolean {
	if (summary?.isAdmin) return true;
	const member = currentTaskMember(summary);
	if (!member) return false;
	if (!task.id && task.requesterID === member.id) return true;
	return task.participantIDs.includes(member.id);
}

export function canDeleteTask(summary: TaskSummary | null, task: Task): boolean {
	if (summary?.isAdmin) return true;
	const member = currentTaskMember(summary);
	return task.participantIDs.length === 1 && member?.id === task.participantIDs[0];
}

export function canManageTaskAssignment(summary: TaskSummary | null, task: Task): boolean {
	if (summary?.isAdmin) return true;
	const member = currentTaskMember(summary);
	return task.participantIDs.length === 1 && member?.id === task.participantIDs[0];
}

export function canRemoveTaskParticipant(task: Task, memberID: string): boolean {
	return Boolean(memberID) && task.participantIDs.length > 1 && task.participantIDs.includes(memberID);
}

function matchesParticipantFilter(task: Task, participantFilterIDs: string[]): boolean {
	if (participantFilterIDs.length === 0) return true;
	return participantFilterIDs.some((memberID) => task.participantIDs.includes(memberID));
}

function matchesDefinitionFilter(value: string | null, filter: string): boolean {
	if (filter === 'all') return true;
	if (filter === ETC_TASK_OPTION_VALUE) return value === null;
	return value === filter;
}

function taskSearchText(task: Task): string {
	return [
		task.content,
		task.ownerName,
		task.business ?? '',
		task.type ?? '',
		...task.participantNames
	].join(' ').toLowerCase();
}

function compareTaskListOrder(left: Task, right: Task): number {
	const statusDifference = statusOrder(left.status) - statusOrder(right.status);
	if (statusDifference !== 0) return statusDifference;

	const dateDifference = compareDescendingDate(taskListSortDate(left), taskListSortDate(right));
	if (dateDifference !== 0) return dateDifference;

	return left.id.localeCompare(right.id);
}

function statusOrder(status: string): number {
	const index = taskListStatusOrder.indexOf(status);
	return index < 0 ? taskListStatusOrder.length : index;
}

function taskListSortDate(task: Task): string {
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
