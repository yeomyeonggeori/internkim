import { browser } from '$app/environment';

export const isEmbeddedFrame = browser && window.self !== window.top;
export const isNestedEmbeddedFrame = browser && window.parent !== window.top;

export function openDetailWindow(url: string): void {
	window.open(url, 'internkim-detail')?.focus();
}
