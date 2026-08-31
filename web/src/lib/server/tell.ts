import type { SupabaseClient } from '@supabase/supabase-js';
import { homePath } from '$lib/home-path';
import type { NotificationCategory } from '$lib/notifications/categories';
import type { Environment } from './agent-request';
import { controlPlane } from './control-plane';
import { notifyMember } from './notify-member';
import type { Notification } from './push-to-member-devices';
import { callCompany, type CompanyCallTransport } from './public-api/company-call';
import type { VapidKeys } from './web-push-vapid';

export const tellCapability = 'person.message.tell';

export type Telling = {
	memberID: string;
	category: NotificationCategory;
	title: string;
	body: string;
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
		pushToTheirDevices(plane, environment, telling),
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
	record: SupabaseClient,
	environment: Environment,
	telling: Telling
): Promise<Attempt> {
	const vapid = vapidKeysOf(environment);
	if (!vapid) return failed(pushChannel, 'this deployment holds no web push keys');
	try {
		const nowInSeconds = Math.floor(Date.now() / 1000);
		const delivery = await notifyMember(
			record,
			telling.memberID,
			telling.category,
			notificationOf(telling),
			vapid,
			nowInSeconds
		);
		return { done: delivery.reached > 0, failure: '' };
	} catch (refusal) {
		return failed(pushChannel, reasonOf(refusal));
	}
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

function notificationOf(telling: Telling): Notification {
	return {
		title: telling.title,
		body: telling.body,
		openPath: homePath,
		tag: `${telling.category}-${crypto.randomUUID()}`
	};
}

function messageOf(telling: Telling): string {
	return [telling.title, telling.body].map((line) => line.trim()).filter((line) => line !== '').join('\n');
}

function controlPlaneOf(environment: Environment): SupabaseClient | null {
	const projectURL = environment.SUPABASE_URL ?? '';
	const serviceRoleKey = environment.SUPABASE_SECRET_KEY ?? environment.SUPABASE_SERVICE_ROLE_KEY ?? '';
	if (!projectURL || !serviceRoleKey) return null;
	return controlPlane({ projectURL, serviceRoleKey });
}

function vapidKeysOf(environment: Environment): VapidKeys | null {
	const publicKey = environment.VAPID_PUBLIC_KEY ?? '';
	const privateKey = environment.VAPID_PRIVATE_KEY ?? '';
	const subject = environment.VAPID_SUBJECT ?? '';
	if (!publicKey || !privateKey || !subject) return null;
	return { publicKey, privateKey, subject };
}

function reasonOf(refusal: unknown): string {
	if (refusal instanceof Error) return refusal.message;
	const said = (refusal as { body?: { message?: unknown } } | null)?.body?.message;
	return typeof said === 'string' ? said : String(refusal);
}
