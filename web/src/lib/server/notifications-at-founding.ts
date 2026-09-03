import type { SupabaseClient } from '@supabase/supabase-js';
import type { Environment } from './agent-request';
import { keepTheAppAddress } from './digest-app-url';
import { askTheProject, type FunctionAnswer } from './project-function';
import type { CompanyCallTransport } from './public-api/company-call';

export type Outcome = 'stored' | 'kept' | 'failed';

export type NotificationsSetUp = { vapid: Outcome; digest: Outcome; appAddress: Outcome; failures: string[] };

type Attempt = { outcome: Outcome; failure: string };

const pushPairFunction = 'setup-vapid';
const digestKeyFunction = 'setup-digest-key';
const appAddressKeeper = 'digest_app_url_keep';

export async function setUpNotificationsFor(
	environment: Environment,
	accessToken: string,
	founderEmail: string,
	plane: SupabaseClient,
	transport?: CompanyCallTransport
): Promise<NotificationsSetUp> {
	const [vapid, digest, appAddress] = await Promise.all([
		keepAPushPairForThem(environment, accessToken, founderEmail, transport),
		keepADigestKeyForThem(environment, accessToken, transport),
		keepTheAppAddressForThem(environment, plane)
	]);
	return {
		vapid: vapid.outcome,
		digest: digest.outcome,
		appAddress: appAddress.outcome,
		failures: [vapid.failure, digest.failure, appAddress.failure].filter((failure) => failure !== '')
	};
}

async function keepAPushPairForThem(
	environment: Environment,
	accessToken: string,
	founderEmail: string,
	transport?: CompanyCallTransport
): Promise<Attempt> {
	try {
		const answer = await askTheProject(
			environment,
			pushPairFunction,
			{ subject: `mailto:${founderEmail}` },
			accessToken,
			transport
		);
		if (answer.status === 409) return { outcome: 'kept', failure: '' };
		return outcomeOf(pushPairFunction, answer);
	} catch (refusal) {
		return failed(pushPairFunction, reasonOf(refusal));
	}
}

async function keepADigestKeyForThem(
	environment: Environment,
	accessToken: string,
	transport?: CompanyCallTransport
): Promise<Attempt> {
	try {
		const answer = await askTheProject(environment, digestKeyFunction, {}, accessToken, transport);
		return outcomeOf(digestKeyFunction, answer);
	} catch (refusal) {
		return failed(digestKeyFunction, reasonOf(refusal));
	}
}

async function keepTheAppAddressForThem(environment: Environment, plane: SupabaseClient): Promise<Attempt> {
	try {
		const stored = await keepTheAppAddress(plane, environment);
		return { outcome: stored ? 'stored' : 'kept', failure: '' };
	} catch (refusal) {
		return failed(appAddressKeeper, reasonOf(refusal));
	}
}

function outcomeOf(functionName: string, answer: FunctionAnswer): Attempt {
	const stored = (answer.body as { stored?: unknown } | null)?.stored;
	if (answer.status === 200 && stored === true) return { outcome: 'stored', failure: '' };
	if (answer.status === 200 && stored === false) return { outcome: 'kept', failure: '' };
	return failed(functionName, refusalOf(answer));
}

function failed(functionName: string, reason: string): Attempt {
	return { outcome: 'failed', failure: `${functionName}: ${reason}` };
}

function refusalOf(answer: FunctionAnswer): string {
	const said = (answer.body as { error?: unknown } | null)?.error;
	return typeof said === 'string' && said ? said : `answered ${answer.status}`;
}

function reasonOf(refusal: unknown): string {
	return refusal instanceof Error ? refusal.message : String(refusal);
}
