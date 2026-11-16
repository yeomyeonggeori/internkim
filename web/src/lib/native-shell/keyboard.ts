import { listenToTauriKeyboard } from './keyboard-tauri';
import { isInsideTauriShell, shellPlatform } from './shell';

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
	if (!isInsideTauriShell() || shellPlatform() !== 'android') return null;
	return listenToTauriKeyboard(onChange);
}
