import { browser } from '$app/environment';

export const appRailModes = ['expanded', 'collapsed', 'hover'] as const;
export type AppRailMode = (typeof appRailModes)[number];

const storageKey = 'internkim.app-rail-mode';
const defaultMode: AppRailMode = 'collapsed';

function isAppRailMode(value: string | null): value is AppRailMode {
	return appRailModes.some((mode) => mode === value);
}

function storedMode(): AppRailMode {
	if (!browser) return defaultMode;
	const value = localStorage.getItem(storageKey);
	return isAppRailMode(value) ? value : defaultMode;
}

class AppRailModeState {
	mode = $state<AppRailMode>(storedMode());
	isPeeking = $state(false);
	isMenuOpen = $state(false);
	isOpen = $derived(this.mode === 'expanded' || (this.mode === 'hover' && this.isPeeking));

	select = (mode: string) => {
		if (!isAppRailMode(mode)) return;
		this.mode = mode;
		if (browser) localStorage.setItem(storageKey, mode);
	};

	setOpen = (open: boolean) => {
		this.select(open ? 'expanded' : 'collapsed');
	};

	setPeeking = (isPeeking: boolean) => {
		this.isPeeking = isPeeking;
	};
}

export const appRailMode = new AppRailModeState();
