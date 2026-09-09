declare global {
	interface Window {
		Capacitor?: { isNativePlatform?: () => boolean; getPlatform?: () => string };
	}
}

export function isInsideNativeShell(): boolean {
	if (typeof window === 'undefined') return false;
	return window.Capacitor?.isNativePlatform?.() === true;
}

export function shellPlatform(): string {
	if (typeof window === 'undefined') return 'web';
	return window.Capacitor?.getPlatform?.() ?? 'web';
}
