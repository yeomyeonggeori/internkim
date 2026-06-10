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
