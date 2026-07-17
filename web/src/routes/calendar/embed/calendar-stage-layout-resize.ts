export function installCalendarStageLayoutResizeSync(
	stageElement: HTMLElement,
	scheduleLayoutSync: () => void
): () => void {
	const layoutResizeDebounceMilliseconds = 50;
	let previousWidth = stageElement.clientWidth;
	let pendingTimeoutID: number | null = null;
	const resizeObserver = new ResizeObserver(() => {
		const currentWidth = stageElement.clientWidth;
		if (currentWidth === previousWidth) return;
		previousWidth = currentWidth;
		if (pendingTimeoutID !== null) window.clearTimeout(pendingTimeoutID);
		pendingTimeoutID = window.setTimeout(() => {
			pendingTimeoutID = null;
			scheduleLayoutSync();
		}, layoutResizeDebounceMilliseconds);
	});
	resizeObserver.observe(stageElement);
	return () => {
		resizeObserver.disconnect();
		if (pendingTimeoutID !== null) window.clearTimeout(pendingTimeoutID);
	};
}
