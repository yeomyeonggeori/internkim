import { isTauri } from '@tauri-apps/api/core';
import { platform as tauriPlatform } from '@tauri-apps/plugin-os';

declare global {
	interface Window {
		Capacitor?: { isNativePlatform?: () => boolean; getPlatform?: () => string };
	}
}

export function isInsideCapacitorShell(): boolean {
	if (typeof window === 'undefined') return false;
	return window.Capacitor?.isNativePlatform?.() === true;
}

export function isInsideTauriShell(): boolean {
	if (typeof window === 'undefined') return false;
	return isTauri();
}

export function shellPlatform(): string {
	if (typeof window === 'undefined') return 'web';
	if (isInsideTauriShell()) return tauriShellPlatform();
	return window.Capacitor?.getPlatform?.() ?? 'web';
}

function tauriShellPlatform(): string {
	if (!('__TAURI_OS_PLUGIN_INTERNALS__' in window)) return 'web';
	return tauriPlatform();
}
