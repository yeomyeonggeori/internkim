import {
	ETC_TASK_OPTION_VALUE,
	buildBusinessFilterOptions,
	buildMemberFilterOptions,
	defaultParticipantFilterIDs,
	filterTasks,
	sortTaskList
} from './task-workspace-model';
import { taskText } from './text';
import type { TaskDefinitions, TaskSummary, Task } from './task-types';
import type { PageText } from '$lib/i18n/page-text.svelte';

type TaskPageText = PageText<typeof taskText>;

const emptyDefinitions: TaskDefinitions = {
	categories: [],
	types: [],
	sizes: []
};

export class TaskFiltersController {
	searchText = $state('');
	statusFilter = $state('all');
	participantFilterIDs = $state<string[]>([]);
	businessFilter = $state('all');
	typeFilter = $state('all');
	private defaultParticipantFilterEmail = '';
	private hasCustomizedParticipantFilter = false;

	sync(summary: TaskSummary | null): void {
		const currentEmail = summary?.currentUserEmail ?? '';
		if (!currentEmail || this.hasCustomizedParticipantFilter && this.defaultParticipantFilterEmail === currentEmail) return;
		if (this.defaultParticipantFilterEmail === currentEmail && this.participantFilterIDs.length > 0) return;
		this.participantFilterIDs = defaultParticipantFilterIDs(summary);
		this.defaultParticipantFilterEmail = currentEmail;
	}

	reset(summary: TaskSummary | null): void {
		this.searchText = '';
		this.statusFilter = 'all';
		this.participantFilterIDs = defaultParticipantFilterIDs(summary);
		this.hasCustomizedParticipantFilter = false;
		this.businessFilter = 'all';
		this.typeFilter = 'all';
	}

	setParticipantIDs(memberIDs: string[]): void {
		this.participantFilterIDs = [...memberIDs];
		this.hasCustomizedParticipantFilter = true;
	}

	statusOptions(summary: TaskSummary | null, text: TaskPageText, statusLabel: (status: string) => string) {
		return [
			{ value: 'all', label: text.filters.all },
			...(summary?.statusOptions ?? []).map((status) => ({ value: status, label: statusLabel(status) }))
		];
	}

	memberOptions(summary: TaskSummary | null, text: TaskPageText) {
		return buildMemberFilterOptions(summary?.members ?? [], text.filters.all);
	}

	businessOptions(summary: TaskSummary | null, text: TaskPageText) {
		return buildBusinessFilterOptions((summary?.definitions ?? emptyDefinitions).categories, text.filters.all, text.report.etcLabel);
	}

	typeOptions(summary: TaskSummary | null, text: TaskPageText) {
		const definitions = summary?.definitions ?? emptyDefinitions;
		return [
			{ value: 'all', label: text.filters.all },
			{ value: ETC_TASK_OPTION_VALUE, label: text.report.etcLabel },
			...definitions.types.map((type) => ({ value: type, label: type }))
		];
	}

	tasks(tasks: Task[]): Task[] {
		return sortTaskList(
			filterTasks(tasks, {
				searchText: this.searchText,
				statusFilter: this.statusFilter,
				participantFilterIDs: this.participantFilterIDs,
				businessFilter: this.businessFilter,
				typeFilter: this.typeFilter
			})
		);
	}
}
