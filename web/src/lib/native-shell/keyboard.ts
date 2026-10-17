import type { PluginListenerHandle } from '@capacitor/core';
import { isInsideCapacitorShell } from './shell';

export type NativeKeyboardWatcher = (onChange: (isOpen: boolean) => void) => () => void;

export const followNativeKeyboard: NativeKeyboardWatcher = (onChange) => {
	if (!isInsideCapacitorShell()) return () => {};
	let isStopped = false;
	let handles: PluginListenerHandle[] = [];
	listenToNativeKeyboard(onChange)
		.then((listening) => {
			handles = listening;
			if (isStopped) removeHandles(handles);
		})
		.catch((failure: unknown) => {
			console.warn('the shell keyboard could not be followed', failure);
		});
	return () => {
		isStopped = true;
		removeHandles(handles);
	};
};

async function listenToNativeKeyboard(onChange: (isOpen: boolean) => void): Promise<PluginListenerHandle[]> {
	const { Capacitor } = await import('@capacitor/core');
	if (!Capacitor.isPluginAvailable('Keyboard')) return [];
	const { Keyboard } = await import('@capacitor/keyboard');
	return Promise.all([
		Keyboard.addListener('keyboardWillShow', () => onChange(true)),
		Keyboard.addListener('keyboardWillHide', () => onChange(false))
	]);
}

function removeHandles(handles: PluginListenerHandle[]): void {
	for (const handle of handles) void handle.remove();
}
