const canAccessWindow = typeof window !== 'undefined';

type FrameWindow = {
	self: unknown;
	top: unknown;
};

export function isEmbeddedFrame(): boolean {
	return canAccessWindow && isFrameEmbedded(window);
}

export function isFrameEmbedded(frameWindow: FrameWindow): boolean {
	return frameWindow.self !== frameWindow.top;
}

export function openDetailWindow(url: string): void {
	window.open(url, 'internkim-detail')?.focus();
}
