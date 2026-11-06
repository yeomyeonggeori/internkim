import { isInsideTauriShell, shellPlatform } from './shell';

export async function showKeyboardAccessoryBar(isShown: boolean): Promise<void> {
	if (!isInsideTauriShell() || shellPlatform() !== 'ios') return;
	const { invoke } = await import('@tauri-apps/api/core');
	await invoke<void>('plugin:keyboard|set_accessory_bar', { isShown });
}

export function hideKeyboardAccessoryBarInside(node: HTMLElement): { destroy(): void } {
	const show = (isShown: boolean): void => {
		showKeyboardAccessoryBar(isShown).catch((failure: unknown) => {
			console.warn('the keyboard accessory bar could not be changed', { isShown, failure });
		});
	};

	const handleFocusIn = (): void => show(false);
	const handleFocusOut = (event: FocusEvent): void => {
		if (event.relatedTarget instanceof Node && node.contains(event.relatedTarget)) return;
		show(true);
	};

	node.addEventListener('focusin', handleFocusIn);
	node.addEventListener('focusout', handleFocusOut);

	return {
		destroy() {
			node.removeEventListener('focusin', handleFocusIn);
			node.removeEventListener('focusout', handleFocusOut);
			if (node.contains(document.activeElement)) show(true);
		}
	};
}
