import { isInsideCapacitorShell, isInsideTauriShell, shellPlatform } from './shell';
import { tauriPageTheme } from './page-theme-tauri';

export type PageTheme = {
	background: string;
	isDark: boolean;
};

export type PageThemePlugin = {
	apply(theme: PageTheme): Promise<void>;
};

export function hexOfComputedColor(computed: string): string | null {
	const channels = computed.match(/\d+/g)?.slice(0, 3).map(Number);
	if (!channels || channels.length !== 3) return null;
	return `#${channels.map((channel) => channel.toString(16).padStart(2, '0')).join('')}`.toUpperCase();
}

function currentPageTheme(): PageTheme | null {
	const background = hexOfComputedColor(getComputedStyle(document.body).backgroundColor);
	if (!background) return null;
	return { background, isDark: document.documentElement.classList.contains('dark') };
}

const mobilePlatforms = ['ios', 'android'];

export async function pageThemeShell(): Promise<PageThemePlugin | null> {
	if (isInsideTauriShell()) return mobilePlatforms.includes(shellPlatform()) ? tauriPageTheme() : null;
	if (!isInsideCapacitorShell()) return null;
	const { registerPlugin } = await import('@capacitor/core');
	return registerPlugin<PageThemePlugin>('PageTheme');
}

export async function keepShellStatusBarOnPageTheme(): Promise<() => void> {
	const shell = await pageThemeShell();
	if (!shell) return () => {};
	const apply = () => {
		const theme = currentPageTheme();
		if (!theme) return;
		shell.apply(theme).catch((failure: unknown) => {
			console.warn('the shell did not take the page theme', theme, failure);
		});
	};
	apply();
	const observer = new MutationObserver(apply);
	observer.observe(document.documentElement, { attributes: true, attributeFilter: ['class'] });
	return () => observer.disconnect();
}
