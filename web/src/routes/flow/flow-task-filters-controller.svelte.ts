import {
	buildBusinessFilterOptions,
	buildMemberFilterOptions,
	defaultParticipantFilterIDs,
	filterFlowTasks,
	sortFlowTaskList
} from './flow-task-workspace-model';
import { flowText } from './text';
import type { FlowDefinitions, FlowSummary, FlowTask } from './flow-types';

type FlowPageText = typeof flowText.ko;

const emptyDefinitions: FlowDefinitions = {
	categories: [],
	types: [],
	sizes: []
};

export class FlowTaskFiltersController {
	searchText = $state('');
	statusFilter = $state('all');
	participantFilterIDs = $state<string[]>([]);
	businessFilter = $state('all');
	typeFilter = $state('all');
	private defaultParticipantFilterEmail = '';
	private hasCustomizedParticipantFilter = false;

	sync(summary: FlowSummary | null): void {
		const currentEmail = summary?.currentUserEmail ?? '';
		if (!currentEmail || this.hasCustomizedParticipantFilter && this.defaultParticipantFilterEmail === currentEmail) return;
		if (this.defaultParticipantFilterEmail === currentEmail && this.participantFilterIDs.length > 0) return;
		this.participantFilterIDs = defaultParticipantFilterIDs(summary);
		this.defaultParticipantFilterEmail = currentEmail;
	}

	reset(summary: FlowSummary | null): void {
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

	statusOptions(summary: FlowSummary | null, text: FlowPageText, statusLabel: (status: string) => string) {
		return [
			{ value: 'all', label: text.filters.all },
			...(summary?.statusOptions ?? []).map((status) => ({ value: status, label: statusLabel(status) }))
		];
	}

	memberOptions(summary: FlowSummary | null, text: FlowPageText) {
		return buildMemberFilterOptions(summary?.members ?? [], text.filters.all);
	}

	businessOptions(summary: FlowSummary | null, text: FlowPageText) {
		return buildBusinessFilterOptions((summary?.definitions ?? emptyDefinitions).categories, text.filters.all, text.report.fallbackBusiness);
	}

	typeOptions(summary: FlowSummary | null, text: FlowPageText) {
		const definitions = summary?.definitions ?? emptyDefinitions;
		return [{ value: 'all', label: text.filters.all }, ...definitions.types.map((type) => ({ value: type, label: type }))];
	}

	tasks(tasks: FlowTask[]): FlowTask[] {
		return sortFlowTaskList(
			filterFlowTasks(tasks, {
				searchText: this.searchText,
				statusFilter: this.statusFilter,
				participantFilterIDs: this.participantFilterIDs,
				businessFilter: this.businessFilter,
				typeFilter: this.typeFilter
			})
		);
	}
}
