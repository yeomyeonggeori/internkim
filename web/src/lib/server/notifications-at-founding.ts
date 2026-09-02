import type { Environment } from './agent-request';
import { askTheProject, type FunctionAnswer } from './project-function';
import type { CompanyCallTransport } from './public-api/company-call';

export type Outcome = 'stored' | 'kept' | 'failed';

export type NotificationsSetUp = { vapid: Outcome; digest: Outcome; failures: string[] };

type Attempt = { outcome: Outcome; failure: string };

const pushPairFunction = 'setup-vapid';
const digestKeyFunction = 'setup-digest-key';

export async function setUpNotificationsFor(
	environment: Environment,
	accessToken: string,
	founderEmail: string,
	transport?: CompanyCallTransport
): Promise<NotificationsSetUp> {
	const [vapid, digest] = await Promise.all([
		keepAPushPairForThem(environment, accessToken, founderEmail, transport),
		keepADigestKeyForThem(environment, accessToken, transport)
	]);
	return {
		vapid: vapid.outcome,
		digest: digest.outcome,
		failures: [vapid.failure, digest.failure].filter((failure) => failure !== '')
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
