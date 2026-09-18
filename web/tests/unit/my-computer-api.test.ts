import { describe, expect, test } from 'bun:test';
import { readCompanionPairing, readMyCompanions } from '../../src/routes/settings/my-computer-api';

describe('the companions the device lists for a member', () => {
	test('keep their id, name, presence and whether they can control the computer', () => {
		const read = readMyCompanions({
			companions: [
				{
					companionID: 'c1',
					displayName: 'laptop',
					isOnline: true,
					lastSeenAt: '2026-09-19T00:00:00Z',
					capabilities: [{ name: 'browser_open' }, { name: 'computer_task' }]
				},
				{ companionID: 'c2', displayName: '', isOnline: false, capabilities: [] }
			]
		});
		expect(read).toEqual([
			{ companionID: 'c1', displayName: 'laptop', isOnline: true, lastSeenAt: '2026-09-19T00:00:00Z', canControlComputer: true },
			{ companionID: 'c2', displayName: 'c2', isOnline: false, lastSeenAt: '', canControlComputer: false }
		]);
	});

	test('refuse an answer that is not a list', () => {
		expect(() => readMyCompanions({ companions: 'none' })).toThrow('not a list');
		expect(() => readMyCompanions({ companions: [{ displayName: 'nameless' }] })).toThrow('no id');
	});
});

describe('a pairing code the device issues', () => {
	test('carries the code and the three commands to run', () => {
		expect(
			readCompanionPairing({
				code: 'ABCD-1234',
				expiresAt: '2026-09-19T00:10:00Z',
				deepLink: 'internkim://pair?code=ABCD-1234',
				installCommand: 'curl -fsSL https://intern.kim/companion/install.sh | sh',
				pairCommand: 'internkim-companion pair --device-url https://device.example.test --code ABCD-1234',
				serviceCommand: 'internkim-companion service install'
			})
		).toEqual({
			code: 'ABCD-1234',
			expiresAt: '2026-09-19T00:10:00Z',
			installCommand: 'curl -fsSL https://intern.kim/companion/install.sh | sh',
			pairCommand: 'internkim-companion pair --device-url https://device.example.test --code ABCD-1234',
			serviceCommand: 'internkim-companion service install'
		});
	});

	test('is refused when a command is missing', () => {
		expect(() => readCompanionPairing({ code: 'ABCD-1234', expiresAt: '2026-09-19T00:10:00Z' })).toThrow('installCommand');
	});
});
