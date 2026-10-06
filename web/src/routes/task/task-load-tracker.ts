export type TaskLoadOptions = {
	reloadState?: boolean;
	preserveActiveTabOnError?: boolean;
};

export type LoadTask = (week: string, options?: TaskLoadOptions) => Promise<boolean>;

export const supersededLoadIsNotAFailure = true;

export type TaskLoadTracker = {
	start: () => number;
	isCurrent: (loadID: number) => boolean;
};

export function createTaskLoadTracker(): TaskLoadTracker {
	let currentLoadID = 0;

	return {
		start: () => {
			currentLoadID += 1;
			return currentLoadID;
		},
		isCurrent: (loadID: number) => loadID === currentLoadID
	};
}
