import { describe, expect, test } from 'bun:test';
import { readFileSync } from 'node:fs';
import {
	hostActionsFor,
	refusalMessageOf,
	stageOfTheUpdate,
	updatePatienceMilliseconds,
	watchOfTheRunningUpdate,
	type UpdateWatch
} from '../../src/lib/host/host-update-model';
import type { HostVersion } from '../../src/lib/host/host-version';
import { companySettingsText } from '../../src/routes/settings/text';

const latest = { version: 'v2026.10.02.090000', publishedAt: '2026-10-02T09:00:00Z', notes: 'Faster replies.' };
const previous = { version: 'v2026.09.30.000000', publishedAt: '2026-09-30T00:00:00Z' };

function hostOn(installedVersion: string, overrides: Partial<HostVersion> = {}): HostVersion {
	return {
		installedVersion,
		channel: 'stable',
		updateMethod: 'apt',
		latestStable: latest,
		previousStable: previous,
		isUpdateAvailable: installedVersion !== latest.version,
		expectedDowntimeSeconds: 60,
		...overrides
	};
}

const startedAt = Date.parse('2026-10-02T14:00:00Z');
const watch: UpdateWatch = { fromVersion: 'v2026.10.01.000000', toVersion: latest.version, startedAt };
const running = { fromVersion: watch.fromVersion, toVersion: watch.toVersion, startedAt: '2026-10-02T14:00:00Z' };

describe('what an administrator may do with the host', () => {
	test('is to update to the latest stable release, or go back to the one before', () => {
		expect(hostActionsFor(hostOn('v2026.10.01.000000'))).toEqual({ kind: 'choices', update: latest, goBack: previous });
	});

	test('offers no update on a host already on the latest, only going back', () => {
		expect(hostActionsFor(hostOn(latest.version))).toEqual({ kind: 'choices', update: null, goBack: previous });
		expect(hostActionsFor(hostOn(latest.version, { previousStable: undefined }))).toEqual({
			kind: 'choices',
			update: null,
			goBack: null
		});
	});

	test('is nothing here on a Mac, an unpackaged machine, or a host off the stable channel', () => {
		expect(hostActionsFor(hostOn('v2026.10.01.000000', { updateMethod: 'brew' })).kind).toBe('brew');
		expect(hostActionsFor(hostOn('v2026.10.01.000000', { updateMethod: '' })).kind).toBe('unpackaged');
		expect(hostActionsFor(hostOn('v2026.10.01.000000', { channel: 'testing' })).kind).toBe('offStable');
		expect(hostActionsFor(hostOn('v2026.10.01.000000', { channel: 'unrecorded' })).kind).toBe('offStable');
	});

	test('is to wait while an update runs, whoever started it', () => {
		const version = hostOn('v2026.10.01.000000', { updateInProgress: running });
		expect(hostActionsFor(version).kind).toBe('updating');
		expect(watchOfTheRunningUpdate(version)).toEqual(watch);
		expect(watchOfTheRunningUpdate(hostOn(latest.version))).toBeNull();
	});
});

describe('an update being watched', () => {
	test('is installing while the old host still answers that it runs', () => {
		const reading = { isAnswered: true as const, version: hostOn(watch.fromVersion, { updateInProgress: running }) };
		expect(stageOfTheUpdate(watch, reading, startedAt + 5000)).toEqual({ kind: 'installing' });
	});

	test('is restarting while the host does not answer, which is not an error', () => {
		expect(stageOfTheUpdate(watch, { isAnswered: false }, startedAt + 30_000)).toEqual({ kind: 'restarting' });
	});

	test('is finished once the host answers on the release it was going to', () => {
		const reading = { isAnswered: true as const, version: hostOn(latest.version) };
		expect(stageOfTheUpdate(watch, reading, startedAt + 90_000)).toEqual({ kind: 'finished' });
	});

	test('has failed when the host answers with nothing running on the release it left, saying why if it knows', () => {
		const lastUpdate = { ...running, finishedAt: '2026-10-02T14:01:00Z', succeeded: false, error: 'install.sh failed: exit status 1' };
		const told = { isAnswered: true as const, version: hostOn(watch.fromVersion, { lastUpdate }) };
		expect(stageOfTheUpdate(watch, told, startedAt + 90_000)).toEqual({ kind: 'failed', reason: lastUpdate.error });
		const untold = { isAnswered: true as const, version: hostOn(watch.fromVersion) };
		expect(stageOfTheUpdate(watch, untold, startedAt + 90_000)).toEqual({ kind: 'failed', reason: '' });
	});

	test('is overdue once the host has been away past the patience, and is still not called failed', () => {
		expect(stageOfTheUpdate(watch, { isAnswered: false }, startedAt + updatePatienceMilliseconds)).toEqual({ kind: 'overdue' });
		const back = { isAnswered: true as const, version: hostOn(latest.version) };
		expect(stageOfTheUpdate(watch, back, startedAt + updatePatienceMilliseconds * 2)).toEqual({ kind: 'finished' });
	});
});

const refusalsTheScreenNeverMeets = ['approval_required', 'invalid_start_time'];

function admindRefusalCodes(): string[] {
	const source = readFileSync('../internal/admind/host_update.go', 'utf8');
	return [...source.matchAll(/hostRefusal\{http\.Status\w+, "(\w+)"/g)].map((match) => match[1]);
}

describe('a refusal of the update', () => {
	test('has words in both languages for every refusal admind can answer the screen with', () => {
		const answerable = admindRefusalCodes().filter((code) => !refusalsTheScreenNeverMeets.includes(code));
		expect(answerable.length).toBeGreaterThan(5);
		expect(Object.keys(companySettingsText.ko.companyHost.refusals).sort()).toEqual([...answerable].sort());
		expect(Object.keys(companySettingsText.en.companyHost.refusals).sort()).toEqual([...answerable].sort());
		expect(admindRefusalCodes()).toEqual(expect.arrayContaining(refusalsTheScreenNeverMeets));
	});

	test('is said in the screen’s words when it knows the code, and in the host’s otherwise', () => {
		const refusals = companySettingsText.en.companyHost.refusals;
		expect(refusalMessageOf('already_installed', refusals, 'this host already runs v1')).toBe(refusals.already_installed);
		expect(refusalMessageOf('something_new', refusals, 'the host said no')).toBe('the host said no');
		expect(refusalMessageOf(undefined, refusals, 'the host said no')).toBe('the host said no');
	});
});
