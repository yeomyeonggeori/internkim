export type AvailableAppUpdate = { version: string; install: () => Promise<void> };

export type AppUpdatePlugin = { check: () => Promise<AvailableAppUpdate | null>; relaunch: () => Promise<void> };

export type AppUpdateOutcome = 'relaunching' | 'failed' | 'nothing-to-install';

export type AppUpdateState = { available: AvailableAppUpdate | null; isInstalling: boolean };

export type AppUpdater = {
	watch: (platform: string, loadPlugin: () => Promise<AppUpdatePlugin>) => () => void;
	check: (loadPlugin: () => Promise<AppUpdatePlugin>) => Promise<void>;
	install: () => Promise<AppUpdateOutcome>;
};

export const appUpdatePlatform = 'macos';

export const appUpdateCheckMilliseconds = 6 * 60 * 60 * 1000;

export function appUpdaterOf(state: AppUpdateState): AppUpdater {
	let plugin: AppUpdatePlugin | null = null;

	async function ask(asked: AppUpdatePlugin): Promise<void> {
		try {
			state.available = await asked.check();
		} catch (failure) {
			console.warn('the app could not ask whether a newer one exists', { failure });
		}
	}

	async function check(loadPlugin: () => Promise<AppUpdatePlugin>): Promise<void> {
		if (state.isInstalling) return;
		try {
			plugin ??= await loadPlugin();
		} catch (failure) {
			console.warn('the app could not reach its updater', { failure });
			return;
		}
		await ask(plugin);
	}

	function watch(platform: string, loadPlugin: () => Promise<AppUpdatePlugin>): () => void {
		if (platform !== appUpdatePlatform) return () => {};
		void check(loadPlugin);
		const timer = setInterval(() => void check(loadPlugin), appUpdateCheckMilliseconds);
		return () => clearInterval(timer);
	}

	async function install(): Promise<AppUpdateOutcome> {
		const update = state.available;
		const installer = plugin;
		if (!update || !installer || state.isInstalling) return 'nothing-to-install';
		state.isInstalling = true;
		try {
			await update.install();
			await installer.relaunch();
			return 'relaunching';
		} catch (failure) {
			console.warn('the app update was not installed', { version: update.version, failure });
			await ask(installer);
			return 'failed';
		} finally {
			state.isInstalling = false;
		}
	}

	return { watch, check, install };
}
