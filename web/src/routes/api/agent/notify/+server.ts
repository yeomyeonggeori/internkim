import { error, json } from '@sveltejs/kit';
import type { SupabaseClient } from '@supabase/supabase-js';
import { callingAgent, environmentOf, type Environment } from '$lib/server/agent-request';
import { membersOfCompanyByExternalID } from '$lib/server/member-credential';
import { notificationCategories, type NotificationCategory } from '$lib/notifications/categories';
import { notifyMember, type Delivery, type Notification } from '$lib/server/notify-member';
import type { VapidKeys } from '$lib/server/web-push-vapid';
import type { RequestHandler } from './$types';

type NotifyRequest = {
	platform?: unknown;
	externalIDs?: unknown;
	category?: unknown;
	title?: unknown;
	body?: unknown;
	openPath?: unknown;
	tag?: unknown;
};

export const POST: RequestHandler = async ({ request, platform }) => {
	const environment = environmentOf(platform);
	const { client, companyID } = await callingAgent(request, environment);
	const vapid = vapidKeys(environment);

	const asked = (await request.json().catch(() => ({}))) as NotifyRequest;
	const recipients = askedExternalIDs(asked.externalIDs);
	const memberOf = await membersOfCompanyByExternalID(client, companyID, askedPlatform(asked.platform));

	const delivered = await tellEach(
		client,
		recipients.map((externalID) => memberOf.get(externalID)).filter((id): id is string => Boolean(id)),
		askedCategory(asked.category),
		askedNotification(asked),
		vapid
	);

	return json({ ...delivered, addressed: recipients.length });
};

async function tellEach(
	client: SupabaseClient,
	memberIDs: string[],
	category: NotificationCategory,
	notification: Notification,
	vapid: VapidKeys
): Promise<{ told: number; reached: number; pruned: number }> {
	const nowInSeconds = Math.floor(Date.now() / 1000);
	const deliveries: Delivery[] = [];
	for (const memberID of memberIDs) {
		deliveries.push(await notifyMember(client, memberID, category, notification, vapid, nowInSeconds));
	}
	return {
		told: deliveries.filter((delivery) => !delivery.silent).length,
		reached: deliveries.reduce((total, delivery) => total + delivery.reached, 0),
		pruned: deliveries.reduce((total, delivery) => total + delivery.pruned, 0)
	};
}

function askedPlatform(offered: unknown): string {
	if (typeof offered !== 'string' || !offered.trim()) error(400, 'which messenger these people are on');
	return offered.trim();
}

function askedCategory(offered: unknown): NotificationCategory {
	const named = notificationCategories.find((category) => category === offered);
	if (!named) error(400, 'that is not a category anything notifies about');
	return named;
}

function askedExternalIDs(offered: unknown): string[] {
	if (!Array.isArray(offered)) error(400, 'externalIDs required');
	return offered.filter((entry): entry is string => typeof entry === 'string' && entry !== '');
}

function askedNotification(asked: NotifyRequest): Notification {
	const title = typeof asked.title === 'string' ? asked.title.trim() : '';
	if (!title) error(400, 'a notification with no title says nothing');
	return {
		title,
		body: typeof asked.body === 'string' ? asked.body : '',
		openPath: typeof asked.openPath === 'string' ? asked.openPath : '/flow/',
		tag: typeof asked.tag === 'string' ? asked.tag : 'internkim'
	};
}

function vapidKeys(environment: Environment): VapidKeys {
	const publicKey = environment.VAPID_PUBLIC_KEY ?? '';
	const privateKey = environment.VAPID_PRIVATE_KEY ?? '';
	const subject = environment.VAPID_SUBJECT ?? '';
	if (!publicKey || !privateKey || !subject) error(503, 'this deployment cannot send notifications yet');
	return { publicKey, privateKey, subject };
}
