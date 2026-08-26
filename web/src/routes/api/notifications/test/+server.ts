import { error, json } from '@sveltejs/kit';
import { environmentOf, type Environment } from '$lib/server/agent-request';
import { callingMember } from '$lib/server/member-request';
import { pushToMemberDevices } from '$lib/server/push-to-member-devices';
import type { VapidKeys } from '$lib/server/web-push-vapid';
import type { RequestHandler } from './$types';

type TestRequest = {
	title?: unknown;
	body?: unknown;
};

const longestLine = 200;

export const POST: RequestHandler = async ({ request, platform }) => {
	const environment = environmentOf(platform);
	const { caller, memberID } = await callingMember(request, environment);
	const vapid = vapidKeys(environment);

	const asked = (await request.json().catch(() => ({}))) as TestRequest;
	const { reached, pruned } = await pushToMemberDevices(
		caller,
		memberID,
		{
			title: line(asked.title) || 'internkim',
			body: line(asked.body),
			openPath: '/settings',
			tag: 'notification-self-test'
		},
		vapid,
		Math.floor(Date.now() / 1000)
	);

	return json({ reached, pruned });
};

function line(offered: unknown): string {
	return typeof offered === 'string' ? offered.trim().slice(0, longestLine) : '';
}

function vapidKeys(environment: Environment): VapidKeys {
	const publicKey = environment.VAPID_PUBLIC_KEY ?? '';
	const privateKey = environment.VAPID_PRIVATE_KEY ?? '';
	const subject = environment.VAPID_SUBJECT ?? '';
	if (!publicKey || !privateKey || !subject) {
		error(503, 'this deployment has no VAPID keys, so it cannot send notifications yet');
	}
	return { publicKey, privateKey, subject };
}
