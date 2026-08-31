import { callingAgent } from '../_shared/agent-caller.ts';
import { notificationCategories, type NotificationCategory } from '../_shared/categories.ts';
import { rememberConversationMembers } from '../_shared/conversation-members.ts';
import { askedObject, json, refuse, serveRefusals } from '../_shared/http.ts';
import { membersOfCompanyByExternalID, pictureURLOfMember } from '../_shared/member-directory.ts';
import { notifyMember, type Delivery, type Notification } from '../_shared/notify-member.ts';
import type { SupabaseClient } from '../_shared/service-client.ts';
import { vapidKeysFromVault } from '../_shared/vapid-from-vault.ts';
import type { VapidKeys } from '../_shared/web-push-vapid.ts';

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

Deno.serve(
	serveRefusals(async (request) => {
		if (request.method !== 'POST') refuse(405, 'POST only');
		const { client, companyID } = await callingAgent(request);
		const vapid = await vapidKeysFromVault(client);
		if (!vapid) refuse(503, 'this deployment cannot send notifications yet');

		const asked = (await askedObject(request)) as NotifyRequest;
		const recipients = askedExternalIDs(asked.externalIDs);
		const memberOf = await membersOfCompanyByExternalID(client, companyID, askedPlatform(asked.platform));

		const sender = typeof asked.senderExternalID === 'string' ? memberOf.get(asked.senderExternalID) : undefined;
		const recipientMemberIDs = recipients
			.map((externalID) => memberOf.get(externalID))
			.filter((id): id is string => Boolean(id));
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
	})
);

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
	if (typeof offered !== 'string' || !offered.trim()) refuse(400, 'which messenger these people are on');
	return offered.trim();
}

function askedCategory(offered: unknown): NotificationCategory {
	const named = notificationCategories.find((category) => category === offered);
	if (!named) refuse(400, 'that is not a category anything notifies about');
	return named;
}

function askedExternalIDs(offered: unknown): string[] {
	if (!Array.isArray(offered)) refuse(400, 'externalIDs required');
	return offered.filter((entry): entry is string => typeof entry === 'string' && entry !== '');
}

function askedNotification(asked: NotifyRequest): Notification {
	const title = typeof asked.title === 'string' ? asked.title.trim() : '';
	if (!title) refuse(400, 'a notification with no title says nothing');
	return {
		title,
		body: typeof asked.body === 'string' ? asked.body : '',
		openPath: typeof asked.openPath === 'string' ? asked.openPath : '/task/',
		tag: typeof asked.tag === 'string' ? asked.tag : 'internkim'
	};
}
