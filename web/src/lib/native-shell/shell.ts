import { isTauri } from '@tauri-apps/api/core';
import { platform as tauriPlatform } from '@tauri-apps/plugin-os';

export function isInsideTauriShell(): boolean {
	if (typeof window === 'undefined') return false;
	return isTauri();
}

export function shellPlatform(): string {
	if (!isInsideTauriShell()) return 'web';
	if (!('__TAURI_OS_PLUGIN_INTERNALS__' in window)) return 'web';
	return tauriPlatform();
}

const phonePlatforms = ['ios', 'android'];

export function isInsideTauriPhoneShell(): boolean {
	return isInsideTauriShell() && phonePlatforms.includes(shellPlatform());
}

const desktopPlatforms = ['macos', 'windows', 'linux'];

export function isInsideTauriDesktopShell(): boolean {
	return isInsideTauriShell() && desktopPlatforms.includes(shellPlatform());
}
