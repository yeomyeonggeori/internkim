import type { SupabaseClient } from '@supabase/supabase-js';
import { homePath } from '$lib/home-path';
import type { NotificationCategory } from '$lib/notifications/categories';
import type { Environment } from './agent-request';
import { controlPlane } from './control-plane';
import { askTheProject, type FunctionAnswer } from './project-function';
import { callCompany, type CompanyCallTransport } from './public-api/company-call';

export const tellCapability = 'person.message.tell';

export type Telling = {
	memberID: string;
	category: NotificationCategory;
	title: string;
	body: string;
	openPath?: string;
	senderMemberID?: string;
};

export type Told = {
	pushed: boolean;
	messaged: boolean;
	failure?: string;
};

type Attempt = { done: boolean; failure: string };

const pushChannel = 'web push';
const messageChannel = 'direct message';

type MemberToTell = { companyID: string; email: string };

export async function tell(
	environment: Environment,
	telling: Telling,
	record?: SupabaseClient,
	transport?: CompanyCallTransport
): Promise<Told> {
	const plane = record ?? controlPlaneOf(environment);
	if (!plane) return { pushed: false, messaged: false, failure: 'the control plane is not configured' };

	const [pushed, messaged] = await Promise.all([
		pushToTheirDevices(environment, telling, transport),
		messageThemOnTheirMessenger(plane, environment, telling, transport)
	]);
	return toldOf(pushed, messaged);
}

function failed(channel: string, reason: string): Attempt {
	return { done: false, failure: `${channel}: ${reason}` };
}

function toldOf(pushed: Attempt, messaged: Attempt): Told {
	const failures = [pushed.failure, messaged.failure].filter((failure) => failure !== '');
	if (failures.length === 0) return { pushed: pushed.done, messaged: messaged.done };
	return { pushed: pushed.done, messaged: messaged.done, failure: failures.join('; ') };
}

async function pushToTheirDevices(
	environment: Environment,
	telling: Telling,
	transport?: CompanyCallTransport
): Promise<Attempt> {
	try {
		const answer = await askTheProject(environment, 'tell-member', {
			memberID: telling.memberID,
			category: telling.category,
			title: telling.title,
			body: telling.body,
			openPath: telling.openPath ?? homePath,
			...(telling.senderMemberID ? { senderMemberID: telling.senderMemberID } : {})
		}, undefined, transport);
		if (answer.status >= 300) return failed(pushChannel, refusalOf(answer));
		const reached = (answer.body as { reached?: unknown } | null)?.reached;
		return { done: typeof reached === 'number' && reached > 0, failure: '' };
	} catch (refusal) {
		return failed(pushChannel, reasonOf(refusal));
	}
}

function refusalOf(answer: FunctionAnswer): string {
	const said = (answer.body as { error?: unknown } | null)?.error;
	return typeof said === 'string' && said ? said : `the project answered ${answer.status}`;
}

async function messageThemOnTheirMessenger(
	record: SupabaseClient,
	environment: Environment,
	telling: Telling,
	transport?: CompanyCallTransport
): Promise<Attempt> {
	try {
		const member = await memberToTell(record, telling.memberID);
		if (!member) return failed(messageChannel, `member ${telling.memberID} has no company and no address`);

		const answer = await callCompany(
			environment,
			member.companyID,
			tellCapability,
			{ recipientEmail: member.email, message: messageOf(telling) },
			transport
		);
		if (answer.status >= 300) return failed(messageChannel, `the company answered ${answer.status}`);
		return { done: true, failure: '' };
	} catch (refusal) {
		return failed(messageChannel, reasonOf(refusal));
	}
}

async function memberToTell(record: SupabaseClient, memberID: string): Promise<MemberToTell | null> {
	const { data, error } = await record
		.from('member')
		.select('company_id, email')
		.eq('id', memberID)
		.maybeSingle<{ company_id: string | null; email: string | null }>();
	if (error) throw new Error(error.message);
	if (!data?.company_id || !data.email) return null;
	return { companyID: data.company_id, email: data.email };
}

function messageOf(telling: Telling): string {
	return [telling.title, telling.body].map((line) => line.trim()).filter((line) => line !== '').join('\n');
}

function controlPlaneOf(environment: Environment): SupabaseClient | null {
	const projectURL = environment.SUPABASE_URL ?? '';
	const serviceRoleKey = environment.SUPABASE_SECRET_KEY ?? '';
	if (!projectURL || !serviceRoleKey) return null;
	return controlPlane({ projectURL, serviceRoleKey });
}

function reasonOf(refusal: unknown): string {
	if (refusal instanceof Error) return refusal.message;
	const said = (refusal as { body?: { message?: unknown } } | null)?.body?.message;
	return typeof said === 'string' ? said : String(refusal);
}
