import { matchesKoreanSearch } from '$lib/korean-search';
import { fetchFlowState } from '../../routes/flow/flow-api';
import { flowBusinessColor, flowTaskTypeColor } from '../../routes/flow/flow-definition-colors';
import type { FlowDefinitions, FlowTask } from '../../routes/flow/flow-types';

const searchResultLimit = 5;

const emptyDefinitions: FlowDefinitions = { categories: [], types: [], sizes: [] };

class FlowTaskSearch {
	tasks = $state<FlowTask[]>([]);
	definitions = $state<FlowDefinitions>(emptyDefinitions);

	load = async () => {
		try {
			const state = await fetchFlowState('');
			this.tasks = state.tasks ?? [];
			this.definitions = state.definitions ?? emptyDefinitions;
		} catch {
			this.tasks = [];
			this.definitions = emptyDefinitions;
		}
	};

	businessColor = (business: string) => flowBusinessColor(business, this.definitions);
	taskTypeColor = (type: string) => flowTaskTypeColor(type, this.definitions);

	search = (query: string): FlowTask[] => {
		const normalizedQuery = query.trim().toLowerCase();
		if (!normalizedQuery) return [];
		const seenContents = new Set<string>();
		const matches: FlowTask[] = [];
		for (const task of this.tasks) {
			if (!matchesKoreanSearch(task.content, normalizedQuery)) continue;
			if (seenContents.has(task.content)) continue;
			seenContents.add(task.content);
			matches.push(task);
			if (matches.length === searchResultLimit) break;
		}
		return matches;
	};
}

export const flowTaskSearch = new FlowTaskSearch();
