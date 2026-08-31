import { error, json } from '@sveltejs/kit';
import type { SupabaseClient } from '@supabase/supabase-js';
import { callingAgent, environmentOf } from '$lib/server/agent-request';
import { membersOfCompanyByExternalID } from '$lib/server/member-credential';
import { notificationCategories, type NotificationCategory } from '$lib/notifications/categories';
import { notifyMember, type Delivery, type Notification } from '$lib/server/notify-member';
import { pictureURLOfMember } from '$lib/server/member-picture-url';
import { rememberConversationMembers } from '$lib/server/conversation-members';
import { vapidKeysInUse } from '$lib/server/vapid-keys';
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
	conversationID?: unknown;
	senderExternalID?: unknown;
};

export const POST: RequestHandler = async ({ request, platform }) => {
	const environment = environmentOf(platform);
	const { client, companyID } = await callingAgent(request, environment);
	const vapid = await vapidKeysInUse(client, environment);
	if (!vapid) error(503, 'this deployment cannot send notifications yet');

	const asked = (await request.json().catch(() => ({}))) as NotifyRequest;
	const recipients = askedExternalIDs(asked.externalIDs);
	const memberOf = await membersOfCompanyByExternalID(client, companyID, askedPlatform(asked.platform));

	const sender = typeof asked.senderExternalID === 'string' ? memberOf.get(asked.senderExternalID) : undefined;
	const recipientMemberIDs = recipients.map((externalID) => memberOf.get(externalID)).filter((id): id is string => Boolean(id));
	const conversationID = typeof asked.conversationID === 'string' ? asked.conversationID : '';
	const delivered = await tellEach(
		client,
		recipientMemberIDs,
		askedCategory(asked.category),
		{ ...askedNotification(asked), icon: await pictureURLOfMember(client, sender ?? '') },
		vapid,
		conversationID
	);
	await rememberConversationMembers(client, conversationID, [...recipientMemberIDs, sender ?? '']);

	return json({ ...delivered, addressed: recipients.length });
};

async function tellEach(
	client: SupabaseClient,
	memberIDs: string[],
	category: NotificationCategory,
	notification: Notification,
	vapid: VapidKeys,
	conversationID: string
): Promise<{ told: number; reached: number; pruned: number }> {
	const nowInSeconds = Math.floor(Date.now() / 1000);
	const deliveries: Delivery[] = [];
	for (const memberID of memberIDs) {
		deliveries.push(await notifyMember(client, memberID, category, notification, vapid, nowInSeconds, conversationID));
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
		openPath: typeof asked.openPath === 'string' ? asked.openPath : '/task/',
		tag: typeof asked.tag === 'string' ? asked.tag : 'internkim'
	};
}
