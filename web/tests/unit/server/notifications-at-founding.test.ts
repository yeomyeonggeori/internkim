import { describe, expect, test } from 'bun:test';
import { setUpNotificationsFor } from '../../../src/lib/server/notifications-at-founding';
import type { CompanyCallTransport } from '../../../src/lib/server/public-api/company-call';

const environment = {
	SUPABASE_URL: 'https://ours.supabase.co',
	SUPABASE_SECRET_KEY: 'a-service-key'
};

const accessToken = 'the-founders-own-token';
const founderEmail = 'founder@example.test';

const pushPairURL = 'https://ours.supabase.co/functions/v1/setup-vapid';
const digestKeyURL = 'https://ours.supabase.co/functions/v1/setup-digest-key';

type Sent = { url: string; authorization: string; body: Record<string, unknown> };

type Answer = { status?: number; said?: unknown; throws?: string };

type Answers = { vapid?: Answer; digest?: Answer };

const bothStored: Answer = { status: 200, said: { stored: true } };

function transportRecording(sent: Sent[], answers: Answers = {}): CompanyCallTransport {
	return async (url, options) => {
		sent.push({
			url,
			authorization: (options.headers as Record<string, string>)?.Authorization ?? '',
			body: JSON.parse(options.body) as Record<string, unknown>
		});

		const answer = (url === pushPairURL ? answers.vapid : answers.digest) ?? bothStored;
		if (answer.throws) throw new Error(answer.throws);
		return { status: answer.status ?? 200, json: async () => answer.said ?? null };
	};
}

function sentTo(sent: Sent[], url: string): Sent[] {
	return sent.filter((one) => one.url === url);
}

describe('setUpNotificationsFor', () => {
	test('a founding that stores both a push pair and a digest key reports both stored', async () => {
		const setUp = await setUpNotificationsFor(environment, accessToken, founderEmail, transportRecording([]));

		expect(setUp).toEqual({ vapid: 'stored', digest: 'stored', failures: [] });
	});

	test('a project that already holds both keeps them, and that is not a failure', async () => {
		const setUp = await setUpNotificationsFor(
			environment,
			accessToken,
			founderEmail,
			transportRecording([], {
				vapid: { status: 200, said: { stored: false } },
				digest: { status: 200, said: { stored: false } }
			})
		);

		expect(setUp).toEqual({ vapid: 'kept', digest: 'kept', failures: [] });
	});

	test('a push pair the devices already carry answers 409, and the standing pair is kept', async () => {
		const setUp = await setUpNotificationsFor(
			environment,
			accessToken,
			founderEmail,
			transportRecording([], { vapid: { status: 409, said: { error: 'a web push device already stands' } } })
		);

		expect(setUp).toEqual({ vapid: 'kept', digest: 'stored', failures: [] });
	});

	test('a digest key the project refuses is reported by name, and the push pair still stands', async () => {
		const setUp = await setUpNotificationsFor(
			environment,
			accessToken,
			founderEmail,
			transportRecording([], { digest: { status: 500, said: { error: 'x' } } })
		);

		expect(setUp).toEqual({
			vapid: 'stored',
			digest: 'failed',
			failures: ['setup-digest-key: x']
		});
	});

	test('a refusal the project does not explain still names the status', async () => {
		const setUp = await setUpNotificationsFor(
			environment,
			accessToken,
			founderEmail,
			transportRecording([], { vapid: { status: 502, said: null } })
		);

		expect(setUp).toEqual({
			vapid: 'failed',
			digest: 'stored',
			failures: ['setup-vapid: answered 502']
		});
	});

	test('a call the runtime never completes is reported with what it said, and the other is still tried', async () => {
		const sent: Sent[] = [];

		const setUp = await setUpNotificationsFor(
			environment,
			accessToken,
			founderEmail,
			transportRecording(sent, { vapid: { throws: 'the project never answered' } })
		);

		expect(setUp).toEqual({
			vapid: 'failed',
			digest: 'stored',
			failures: ['setup-vapid: the project never answered']
		});
		expect(sentTo(sent, digestKeyURL)).toHaveLength(1);
	});

	test('the push pair is asked for under the founder own address, and the digest key asks for nothing', async () => {
		const sent: Sent[] = [];

		await setUpNotificationsFor(environment, accessToken, founderEmail, transportRecording(sent));

		expect(sentTo(sent, pushPairURL)).toHaveLength(1);
		expect(sentTo(sent, pushPairURL)[0]?.body).toEqual({ subject: 'mailto:founder@example.test' });
		expect(sentTo(sent, digestKeyURL)).toHaveLength(1);
		expect(sentTo(sent, digestKeyURL)[0]?.body).toEqual({});
	});

	test('both calls are signed with the founder own token rather than the key the project holds', async () => {
		const sent: Sent[] = [];

		await setUpNotificationsFor(environment, accessToken, founderEmail, transportRecording(sent));

		expect(sent.map((one) => one.url).sort()).toEqual([digestKeyURL, pushPairURL]);
		expect(sent.map((one) => one.authorization)).toEqual([
			'Bearer the-founders-own-token',
			'Bearer the-founders-own-token'
		]);
	});
});
