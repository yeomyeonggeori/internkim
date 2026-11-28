import type { AppUpdatePlugin } from './app-update';

export async function tauriAppUpdate(): Promise<AppUpdatePlugin> {
	const { check } = await import('@tauri-apps/plugin-updater');
	const { relaunch } = await import('@tauri-apps/plugin-process');
	return {
		check: async () => {
			const update = await check();
			if (!update) return null;
			return { version: update.version, install: () => update.downloadAndInstall() };
		},
		relaunch
	};
}
