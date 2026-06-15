const moduleLoadRecoveryStorageKey = 'internkim.module-load-recovery';
const moduleLoadRetryWindowMilliseconds = 60_000;

type ModuleLoadRecoveryStorage = Pick<Storage, 'getItem' | 'setItem'>;

type ModuleLoadRecoveryEnvironment = {
	currentPath: string;
	now: () => number;
	reload: () => void;
	schedule: (callback: () => void) => unknown;
	storage: ModuleLoadRecoveryStorage;
};

export function scheduleModuleLoadRecovery(error: unknown, message: string, environment = browserModuleLoadRecoveryEnvironment()) {
	if (!isModuleLoadError(error, message)) return false;
	if (!claimModuleLoadRetry(environment.currentPath, environment.storage, environment.now())) return false;
	environment.schedule(environment.reload);
	return true;
}

export function isModuleLoadError(error: unknown, message = '') {
	const candidates = [message, errorMessage(error), errorStack(error)];
	return candidates.some(containsModuleLoadErrorText);
}

export function claimModuleLoadRetry(currentPath: string, storage: ModuleLoadRecoveryStorage, now: number) {
	try {
		if (hasRecentModuleLoadRetry(currentPath, storage.getItem(moduleLoadRecoveryStorageKey), now)) return false;
		storage.setItem(moduleLoadRecoveryStorageKey, `${currentPath}|${now}`);
		return true;
	} catch {
		return false;
	}
}

function containsModuleLoadErrorText(value: string) {
	const normalizedValue = value.toLowerCase();
	return moduleLoadErrorPhrases.some((phrase) => normalizedValue.includes(phrase));
}

const moduleLoadErrorPhrases = [
	'importing a module script failed',
	'failed to fetch dynamically imported module',
	'error loading dynamically imported module',
	'failed to load module script',
	'chunkloaderror',
	'loading chunk',
	'unable to preload css'
];

function errorMessage(error: unknown) {
	if (error instanceof Error) return error.message;
	if (typeof error === 'string') return error;
	return '';
}

function errorStack(error: unknown) {
	if (error instanceof Error) return error.stack ?? '';
	return '';
}

function hasRecentModuleLoadRetry(currentPath: string, record: string | null, now: number) {
	if (!record) return false;
	const delimiterIndex = record.lastIndexOf('|');
	if (delimiterIndex < 0) return false;
	const recordedPath = record.slice(0, delimiterIndex);
	const recordedAt = Number(record.slice(delimiterIndex + 1));
	if (!Number.isFinite(recordedAt)) return false;
	return recordedPath === currentPath && now - recordedAt < moduleLoadRetryWindowMilliseconds;
}

function browserModuleLoadRecoveryEnvironment(): ModuleLoadRecoveryEnvironment {
	return {
		currentPath: `${window.location.pathname}${window.location.search}`,
		now: Date.now,
		reload: () => window.location.reload(),
		schedule: (callback) => window.setTimeout(callback, 0),
		storage: window.sessionStorage
	};
}
