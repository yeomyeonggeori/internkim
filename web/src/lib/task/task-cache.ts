export type CachedTask = { id: string; updated_at: string };

export type TaskCacheEntry<Task extends CachedTask> = {
	tasks: Task[];
	fetchedAt: string;
};

let held: TaskCacheEntry<CachedTask> | null = null;

export function heldTasks<Task extends CachedTask>(): TaskCacheEntry<Task> | null {
	return held as TaskCacheEntry<Task> | null;
}

export function holdTasks<Task extends CachedTask>(entry: TaskCacheEntry<Task>): void {
	held = entry;
}

export function forgetHeldTasks(): void {
	held = null;
}

export function mergeChangedTasks<Task extends CachedTask>(
	cached: Task[],
	changed: Task[],
	liveIDs: Set<string>
): Task[] {
	const byID = new Map(cached.filter((task) => liveIDs.has(task.id)).map((task) => [task.id, task]));
	for (const task of changed) byID.set(task.id, task);
	return [...byID.values()];
}

export function newestStamp<Task extends CachedTask>(tasks: Task[]): string {
	return tasks.reduce((newest, task) => (task.updated_at > newest ? task.updated_at : newest), '');
}
