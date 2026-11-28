import { afterEach, describe, expect, jest, test } from 'bun:test';
import {
	appUpdateCheckMilliseconds,
	appUpdaterOf,
	type AppUpdatePlugin,
	type AppUpdateState,
	type AppUpdater
} from '../../../src/lib/native-shell/app-update';

type Recorded = { checks: number; installs: number; relaunches: number };

function pluginOffering(version: string | null, recorded: Recorded, install: () => Promise<void> = async () => {}): () => Promise<AppUpdatePlugin> {
	return async () => ({
		check: async () => {
			recorded.checks += 1;
			if (version === null) return null;
			return {
				version,
				install: async () => {
					recorded.installs += 1;
					await install();
				}
			};
		},
		relaunch: async () => {
			recorded.relaunches += 1;
		}
	});
}

function freshUpdate(): { state: AppUpdateState; updater: AppUpdater } {
	const state: AppUpdateState = { available: null, isInstalling: false };
	return { state, updater: appUpdaterOf(state) };
}

function nothingRecorded(): Recorded {
	return { checks: 0, installs: 0, relaunches: 0 };
}

async function settled(): Promise<void> {
	await Promise.resolve();
	await Promise.resolve();
	await Promise.resolve();
}

afterEach(() => {
	jest.useRealTimers();
});

describe('the Mac app updating itself', () => {
	test('asks nothing outside the Mac app', async () => {
		for (const platform of ['web', 'ios', 'android', 'windows', 'linux']) {
			const recorded = nothingRecorded();
			const { state, updater } = freshUpdate();
			updater.watch(platform, pluginOffering('1.2.0', recorded))();
			await settled();
			expect(recorded.checks).toBe(0);
			expect(state.available).toBeNull();
		}
	});

	test('offers the newer version it is told about, and asks again every six hours', async () => {
		jest.useFakeTimers();
		const recorded = nothingRecorded();
		const { state, updater } = freshUpdate();
		const stop = updater.watch('macos', pluginOffering('1.2.0', recorded));
		await settled();
		expect(state.available?.version).toBe('1.2.0');
		expect(recorded.checks).toBe(1);
		jest.advanceTimersByTime(appUpdateCheckMilliseconds);
		await settled();
		expect(recorded.checks).toBe(2);
		stop();
		jest.advanceTimersByTime(appUpdateCheckMilliseconds);
		await settled();
		expect(recorded.checks).toBe(2);
	});

	test('offers nothing when the app is the newest', async () => {
		const { state, updater } = freshUpdate();
		await updater.check(pluginOffering(null, nothingRecorded()));
		expect(state.available).toBeNull();
		expect(await updater.install()).toBe('nothing-to-install');
	});

	test('installs, then relaunches, and shows it is installing meanwhile', async () => {
		const recorded = nothingRecorded();
		const { state, updater } = freshUpdate();
		let finishInstalling = () => {};
		const installing = new Promise<void>((resolve) => {
			finishInstalling = resolve;
		});
		await updater.check(pluginOffering('1.2.0', recorded, () => installing));
		const outcome = updater.install();
		await settled();
		expect(state.isInstalling).toBe(true);
		expect(await updater.install()).toBe('nothing-to-install');
		expect(recorded.relaunches).toBe(0);
		finishInstalling();
		expect(await outcome).toBe('relaunching');
		expect(recorded).toEqual({ checks: 1, installs: 1, relaunches: 1 });
	});

	test('asks again and keeps offering the update when installing it fails', async () => {
		const recorded = nothingRecorded();
		const { state, updater } = freshUpdate();
		const warn = console.warn;
		console.warn = () => {};
		try {
			await updater.check(pluginOffering('1.2.0', recorded, async () => Promise.reject(new Error('the download stopped'))));
			expect(await updater.install()).toBe('failed');
		} finally {
			console.warn = warn;
		}
		expect(state.isInstalling).toBe(false);
		expect(state.available?.version).toBe('1.2.0');
		expect(recorded).toEqual({ checks: 2, installs: 1, relaunches: 0 });
	});

	test('keeps what it last knew when asking fails', async () => {
		const { state, updater } = freshUpdate();
		await updater.check(pluginOffering('1.2.0', nothingRecorded()));
		const warn = console.warn;
		console.warn = () => {};
		try {
			await updater.check(async () => Promise.reject(new Error('unused: the plugin is already loaded')));
			const failing = freshUpdate();
			await failing.updater.check(async () => Promise.reject(new Error('offline')));
			expect(failing.state.available).toBeNull();
		} finally {
			console.warn = warn;
		}
		expect(state.available?.version).toBe('1.2.0');
	});
});
