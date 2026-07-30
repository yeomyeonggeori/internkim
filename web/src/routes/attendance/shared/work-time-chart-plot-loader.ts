import type { Component } from 'svelte';

type WorkTimeChartPlot = Component<Record<string, unknown>>;

let loadedPlot: WorkTimeChartPlot | undefined;
let pendingPlot: Promise<WorkTimeChartPlot> | undefined;

export function loadedWorkTimeChartPlot(): WorkTimeChartPlot | undefined {
	return loadedPlot;
}

export function loadWorkTimeChartPlot(): Promise<WorkTimeChartPlot> {
	pendingPlot ??= import('./work-time-chart-plot.svelte').then((module) => {
		loadedPlot = module.default as WorkTimeChartPlot;
		return loadedPlot;
	});
	return pendingPlot;
}

export function preloadWorkTimeChartPlot(): void {
	if (typeof window === 'undefined' || loadedPlot || pendingPlot) return;
	const startLoading = () => void loadWorkTimeChartPlot();
	const idleCallback = window.requestIdleCallback;
	if (idleCallback) {
		idleCallback(startLoading);
		return;
	}
	setTimeout(startLoading, 500);
}
