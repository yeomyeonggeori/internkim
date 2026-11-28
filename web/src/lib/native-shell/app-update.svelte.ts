import { appUpdaterOf, type AppUpdateState } from './app-update';

const state = $state<AppUpdateState>({ available: null, isInstalling: false });

export const appUpdate = { state, ...appUpdaterOf(state) };
