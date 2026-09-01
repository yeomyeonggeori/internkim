import { error, json } from '@sveltejs/kit';
import { askedObject } from '$lib/server/asked-object';
import { environmentOf } from '$lib/server/agent-request';
import { callingMember } from '$lib/server/member-request';
import { pushToMemberDevices } from '$lib/server/push-to-member-devices';
import { vapidKeysInUse } from '$lib/server/vapid-keys';
import type { RequestHandler } from './$types';

type TestRequest = {
	title?: unknown;
	body?: unknown;
};

const longestLine = 200;

export const POST: RequestHandler = async ({ request, platform }) => {
	const environment = environmentOf(platform);
	const { caller, record, memberID } = await callingMember(request, environment);
	const vapid = await vapidKeysInUse(record, environment);
	if (!vapid) error(503, 'this deployment has no VAPID keys, so it cannot send notifications yet');

	const asked = (await askedObject(request)) as TestRequest;
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
