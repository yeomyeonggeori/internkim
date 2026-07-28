import { matchesKoreanSearch } from '$lib/korean-search';
import { fetchFlowState } from '../../routes/flow/flow-api';
import type { FlowTask } from '../../routes/flow/flow-types';

const searchResultLimit = 5;

class FlowTaskSearch {
	tasks = $state<FlowTask[]>([]);

	load = async () => {
		try {
			const state = await fetchFlowState('');
			this.tasks = state.tasks ?? [];
		} catch {
			this.tasks = [];
		}
	};

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
