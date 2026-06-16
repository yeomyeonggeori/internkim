export type FlowLoadOptions = {
	reloadState?: boolean;
	preserveActiveTabOnError?: boolean;
};

export type LoadFlow = (week: string, options?: FlowLoadOptions) => Promise<boolean>;

export type FlowLoadTracker = {
	start: () => number;
	isCurrent: (loadID: number) => boolean;
};

export function createFlowLoadTracker(): FlowLoadTracker {
	let currentLoadID = 0;

	return {
		start: () => {
			currentLoadID += 1;
			return currentLoadID;
		},
		isCurrent: (loadID: number) => loadID === currentLoadID
	};
}
