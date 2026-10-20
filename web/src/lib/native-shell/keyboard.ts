import type { PluginListenerHandle } from '@capacitor/core';
import { listenToTauriKeyboard } from './keyboard-tauri';
import { isInsideCapacitorShell, isInsideTauriShell, shellPlatform } from './shell';

export type NativeKeyboardWatcher = (onChange: (isOpen: boolean) => void) => () => void;

export const followNativeKeyboard: NativeKeyboardWatcher = (onChange) => {
	const listening = nativeKeyboardListener(onChange);
	if (!listening) return () => {};
	let isStopped = false;
	let stopListening = () => {};
	listening
		.then((stop) => {
			stopListening = stop;
			if (isStopped) stop();
		})
		.catch((failure: unknown) => {
			console.warn('the shell keyboard could not be followed', failure);
		});
	return () => {
		isStopped = true;
		stopListening();
	};
};

function nativeKeyboardListener(onChange: (isOpen: boolean) => void): Promise<() => void> | null {
	if (isInsideTauriShell()) return shellPlatform() === 'android' ? listenToTauriKeyboard(onChange) : null;
	if (!isInsideCapacitorShell()) return null;
	return listenToCapacitorKeyboard(onChange).then((handles) => () => removeHandles(handles));
}

async function listenToCapacitorKeyboard(onChange: (isOpen: boolean) => void): Promise<PluginListenerHandle[]> {
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
