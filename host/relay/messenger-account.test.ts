import { describe, expect, test } from 'bun:test';
import { keepMessengerAccount } from './messenger-account';
import { RefusedAccount, type MattermostSettings } from './mattermost';

function connectionOf(password: string): MattermostSettings {
	return { baseURL: 'https://messenger.example.test', email: 'sample@example.test', password };
}

function aMessengerThat(answer: (settings: MattermostSettings) => Promise<{ token: string; userID: string }>) {
	const attempts: string[] = [];
	return {
		attempts,
		signInAs: async (settings: MattermostSettings) => {
			attempts.push(settings.password);
			return answer(settings);
		}
	};
}

const welcomes = () => aMessengerThat(async () => ({ token: 'session-token', userID: 'U-admin' }));
const refuses = () => aMessengerThat(async () => Promise.reject(new RefusedAccount(401)));

describe('the account the relay keeps on the messenger', () => {
	test('signs in once and answers from the session it holds', async () => {
		const messenger = welcomes();
		const keeper = keepMessengerAccount(async () => connectionOf('right'), messenger.signInAs);

		const first = await keeper.admin();
		const second = await keeper.admin();

		expect(first.session.token).toBe('session-token');
		expect(second).toBe(first);
		expect(messenger.attempts).toEqual(['right']);
	});

	test('stops asking after the messenger refuses the account, so it is never locked out', async () => {
		const messenger = refuses();
		const keeper = keepMessengerAccount(async () => connectionOf('wrong'), messenger.signInAs);

		await expect(keeper.admin()).rejects.toThrow('mattermost refused this account (401)');
		for (let attempt = 0; attempt < 20; attempt += 1) {
			await expect(keeper.admin()).rejects.toThrow('asked again when the password changes');
		}

		expect(messenger.attempts).toEqual(['wrong']);
	});

	test('asks again once the recorded password changes', async () => {
		let password = 'wrong';
		const attempts: string[] = [];
		const keeper = keepMessengerAccount(
			async () => connectionOf(password),
			async (settings) => {
				attempts.push(settings.password);
				if (settings.password === 'wrong') throw new RefusedAccount(401);
				return { token: 'session-token', userID: 'U-admin' };
			}
		);

		await expect(keeper.admin()).rejects.toThrow('mattermost refused this account (401)');
		await expect(keeper.admin()).rejects.toThrow('asked again when the password changes');
		password = 'right';

		expect((await keeper.admin()).session.token).toBe('session-token');
		expect(attempts).toEqual(['wrong', 'right']);
	});

	test('keeps asking after a failure that is not a refusal, because that one passes', async () => {
		let reachable = false;
		const attempts: string[] = [];
		const keeper = keepMessengerAccount(
			async () => connectionOf('right'),
			async (settings) => {
				attempts.push(settings.password);
				if (!reachable) throw new Error('mattermost login returned 502');
				return { token: 'session-token', userID: 'U-admin' };
			}
		);

		await expect(keeper.admin()).rejects.toThrow('502');
		await expect(keeper.admin()).rejects.toThrow('502');
		reachable = true;

		expect((await keeper.admin()).session.token).toBe('session-token');
		expect(attempts).toEqual(['right', 'right', 'right']);
	});

	test('signs in again after the session it held stopped working', async () => {
		const messenger = welcomes();
		const keeper = keepMessengerAccount(async () => connectionOf('right'), messenger.signInAs);

		await keeper.admin();
		keeper.forgetSession();
		await keeper.admin();

		expect(messenger.attempts).toEqual(['right', 'right']);
	});
});

describe('the address the messenger answers at', () => {
	test('is read once and then held, because a session is not needed to know it', async () => {
		let reads = 0;
		const keeper = keepMessengerAccount(async () => {
			reads += 1;
			return connectionOf('right');
		}, refuses().signInAs);

		expect((await keeper.address()).baseURL).toBe('https://messenger.example.test');
		expect((await keeper.address()).baseURL).toBe('https://messenger.example.test');
		expect(reads).toBe(1);
	});

	test('is asked for again when the central plane could not answer', async () => {
		let answers = false;
		const keeper = keepMessengerAccount(async () => {
			if (!answers) throw new Error('the central plane answered 503 for the mattermost connection');
			return connectionOf('right');
		}, welcomes().signInAs);

		await expect(keeper.address()).rejects.toThrow('503');
		answers = true;

		expect((await keeper.address()).email).toBe('sample@example.test');
	});
});
