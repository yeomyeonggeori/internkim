import type { PageTheme, PageThemePlugin } from './page-theme';

export async function tauriPageTheme(): Promise<PageThemePlugin> {
	const { invoke } = await import('@tauri-apps/api/core');
	return { apply: (theme: PageTheme) => invoke<void>('plugin:page-theme|apply', { ...theme }) };
}
