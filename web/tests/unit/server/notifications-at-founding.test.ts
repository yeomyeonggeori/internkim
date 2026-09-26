import { describe, expect, test } from 'bun:test';
import type { SupabaseClient } from '@supabase/supabase-js';
import { setUpNotificationsFor } from '../../../src/lib/server/notifications-at-founding';
import type { CompanyCallTransport } from '../../../src/lib/server/public-api/company-call';

const environment = {
	SUPABASE_URL: 'https://ours.supabase.co',
	SUPABASE_SECRET_KEY: 'a-service-key',
	CLOUDFLARE_DOMAIN: 'ours.example.test'
};

const accessToken = 'the-founders-own-token';
const founderEmail = 'founder@example.test';

const pushPairURL = 'https://ours.supabase.co/functions/v1/setup-vapid';

type Sent = { url: string; authorization: string; body: Record<string, unknown> };

type Answer = { status?: number; said?: unknown; throws?: string };

type Answers = { vapid?: Answer };

type Asked = { name: string; arguments: unknown };

type Keeping = { data?: unknown; error?: { message: string }; throws?: string };

const stored: Answer = { status: 200, said: { stored: true } };

function transportRecording(sent: Sent[], answers: Answers = {}): CompanyCallTransport {
	return async (url, options) => {
		sent.push({
			url,
			authorization: (options.headers as Record<string, string>)?.Authorization ?? '',
			body: JSON.parse(options.body) as Record<string, unknown>
		});

		const answer = (url === pushPairURL ? answers.vapid : undefined) ?? stored;
		if (answer.throws) throw new Error(answer.throws);
		return { status: answer.status ?? 200, json: async () => answer.said ?? null };
	};
}

function planeRecording(asked: Asked[] = [], keeping: Keeping = { data: true }): SupabaseClient {
	return {
		rpc: async (name: string, args: unknown) => {
			asked.push({ name, arguments: args });
			if (keeping.throws) throw new Error(keeping.throws);
			return { data: keeping.data ?? null, error: keeping.error ?? null };
		}
	} as unknown as SupabaseClient;
}

function sentTo(sent: Sent[], url: string): Sent[] {
	return sent.filter((one) => one.url === url);
}

describe('setUpNotificationsFor', () => {
	test('a founding that stores a push pair, the project address and the app address reports all three stored', async () => {
		const setUp = await setUpNotificationsFor(
			environment,
			accessToken,
			founderEmail,
			planeRecording(),
			transportRecording([])
		);

		expect(setUp).toEqual({ vapid: 'stored', projectAddress: 'stored', appAddress: 'stored', failures: [] });
	});

	test('a project that already holds the push pair and the app address keeps them, and that is not a failure', async () => {
		const setUp = await setUpNotificationsFor(
			environment,
			accessToken,
			founderEmail,
			planeRecording([], { data: false }),
			transportRecording([], { vapid: { status: 200, said: { stored: false } } })
		);

		expect(setUp).toEqual({ vapid: 'kept', projectAddress: 'stored', appAddress: 'kept', failures: [] });
	});

	test('a push pair the devices already carry answers 409, and the standing pair is kept', async () => {
		const setUp = await setUpNotificationsFor(
			environment,
			accessToken,
			founderEmail,
			planeRecording(),
			transportRecording([], { vapid: { status: 409, said: { error: 'a web push device already stands' } } })
		);

		expect(setUp).toEqual({ vapid: 'kept', projectAddress: 'stored', appAddress: 'stored', failures: [] });
	});

	test('a refusal the project does not explain still names the status', async () => {
		const setUp = await setUpNotificationsFor(
			environment,
			accessToken,
			founderEmail,
			planeRecording(),
			transportRecording([], { vapid: { status: 502, said: null } })
		);

		expect(setUp).toEqual({
			vapid: 'failed',
			projectAddress: 'stored',
			appAddress: 'stored',
			failures: ['setup-vapid: answered 502']
		});
	});

	test('a call the runtime never completes is reported with what it said, and the addresses are still kept', async () => {
		const asked: Asked[] = [];

		const setUp = await setUpNotificationsFor(
			environment,
			accessToken,
			founderEmail,
			planeRecording(asked),
			transportRecording([], { vapid: { throws: 'the project never answered' } })
		);

		expect(setUp).toEqual({
			vapid: 'failed',
			projectAddress: 'stored',
			appAddress: 'stored',
			failures: ['setup-vapid: the project never answered']
		});
		expect(asked).toHaveLength(2);
	});

	test('the push pair is asked for under the founder own address and signed with the founder own token', async () => {
		const sent: Sent[] = [];

		await setUpNotificationsFor(environment, accessToken, founderEmail, planeRecording(), transportRecording(sent));

		expect(sent.map((one) => one.url)).toEqual([pushPairURL]);
		expect(sent[0]?.body).toEqual({ subject: 'mailto:founder@example.test' });
		expect(sent[0]?.authorization).toBe('Bearer the-founders-own-token');
	});

	test('the addresses are the project and the zone the app answers on, and nothing the founder sent', async () => {
		const asked: Asked[] = [];

		await setUpNotificationsFor(environment, accessToken, founderEmail, planeRecording(asked), transportRecording([]));

		expect(asked).toEqual([
			{ name: 'digest_target_keep', arguments: { new_project_url: 'https://ours.supabase.co' } },
			{ name: 'digest_app_url_keep', arguments: { new_app_url: 'https://ours.example.test' } }
		]);
	});

	test('a deployment that names no zone falls back to the one the build carries', async () => {
		const asked: Asked[] = [];

		await setUpNotificationsFor(
			{ SUPABASE_URL: environment.SUPABASE_URL, SUPABASE_SECRET_KEY: environment.SUPABASE_SECRET_KEY },
			accessToken,
			founderEmail,
			planeRecording(asked),
			transportRecording([])
		);

		expect(asked[1]?.arguments).toEqual({ new_app_url: 'https://intern.kim' });
	});

	test('a plane that refuses the addresses reports each by name, and the push pair still stands', async () => {
		const setUp = await setUpNotificationsFor(
			environment,
			accessToken,
			founderEmail,
			planeRecording([], { error: { message: 'permission denied' } }),
			transportRecording([])
		);

		expect(setUp).toEqual({
			vapid: 'stored',
			projectAddress: 'failed',
			appAddress: 'failed',
			failures: ['digest_target_keep: permission denied', 'digest_app_url_keep: permission denied']
		});
	});
});
