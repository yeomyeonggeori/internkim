import { matchesKoreanSearch } from '$lib/korean-search';
import { fetchTaskState } from '../../routes/task/task-api';
import { taskBusinessColor, taskTypeColor } from '../../routes/task/task-definition-colors';
import type { TaskDefinitions, Task } from '../../routes/task/task-types';

const searchResultLimit = 5;

const emptyDefinitions: TaskDefinitions = { categories: [], types: [], sizes: [] };

class TaskSearch {
	tasks = $state<Task[]>([]);
	definitions = $state<TaskDefinitions>(emptyDefinitions);

	load = async () => {
		try {
			const state = await fetchTaskState('');
			this.tasks = state.tasks ?? [];
			this.definitions = state.definitions ?? emptyDefinitions;
		} catch {
			this.tasks = [];
			this.definitions = emptyDefinitions;
		}
	};

	businessColor = (business: string) => taskBusinessColor(business, this.definitions);
	taskTypeColor = (type: string) => taskTypeColor(type, this.definitions);

	search = (query: string): Task[] => {
		const normalizedQuery = query.trim().toLowerCase();
		if (!normalizedQuery) return [];
		const seenContents = new Set<string>();
		const matches: Task[] = [];
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

export const taskSearch = new TaskSearch();
