import type { TaskReach } from './categories.ts';
import { isWithinReach, type Placed } from './colleague-reach.ts';

export type TaskListener = Placed & {
	taskReach: TaskReach;
};

export function whoHearsTheCompletionOf(finisherID: string, colleagues: TaskListener[]): string[] {
	const colleagueByID = new Map(colleagues.map((colleague) => [colleague.id, colleague]));
	const finisher = colleagueByID.get(finisherID);
	if (!finisher) return [];
	return colleagues
		.filter((listener) => listener.id !== finisherID && hears(listener, finisher, colleagueByID))
		.map((listener) => listener.id);
}

function hears(listener: TaskListener, finisher: TaskListener, colleagueByID: Map<string, TaskListener>): boolean {
	if (listener.taskReach === 'involved') return false;
	return isWithinReach(listener.taskReach, listener, finisher, colleagueByID);
}
