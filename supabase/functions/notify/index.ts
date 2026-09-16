import { callingAgent } from '../_shared/agent-caller.ts';
import { notificationCategories, type NotificationCategory } from '../_shared/categories.ts';
import { rememberConversationMembers } from '../_shared/conversation-members.ts';
import { askedObject, json, refuse, serveRefusals } from '../_shared/http.ts';
import { membersOfCompanyByEmail, membersOfCompanyByExternalID } from '../_shared/member-directory.ts';
import { addressedIn } from '../_shared/notify-recipients.ts';
import { pictureURLOfSender } from '../_shared/sender-picture.ts';
import { notifyMember, type Delivery, type Notification } from '../_shared/notify-member.ts';
import type { SupabaseClient } from '../_shared/service-client.ts';
import { pushKeysFromVault, reachesSomeDevice } from '../_shared/push-keys.ts';
import type { PushKeys } from '../_shared/push-keys.ts';

type NotifyRequest = {
	platform?: unknown;
	externalIDs?: unknown;
	emails?: unknown;
	category?: unknown;
	title?: unknown;
	body?: unknown;
	openPath?: unknown;
	tag?: unknown;
	conversationID?: unknown;
	senderExternalID?: unknown;
	senderPicturePath?: unknown;
};

Deno.serve(
	serveRefusals(async (request) => {
		if (request.method !== 'POST') refuse(405, 'POST only');
		const { client, companyID } = await callingAgent(request);
		const pushKeys = await pushKeysFromVault(client);
		if (!reachesSomeDevice(pushKeys)) refuse(503, 'this deployment cannot send notifications yet');

		const asked = (await askedObject(request)) as NotifyRequest;
		const addressed = addressedIn(asked);
		if (!addressed) refuse(400, 'who to tell: emails, or a platform and the externalIDs of its accounts');
		const recipients = addressed.keys;
		const memberOf =
			addressed.by === 'email'
				? await membersOfCompanyByEmail(client, companyID)
				: await membersOfCompanyByExternalID(client, companyID, addressed.platform);

		const sender = typeof asked.senderExternalID === 'string' ? memberOf.get(asked.senderExternalID) : undefined;
		const recipientMemberIDs = recipients
			.map((key) => memberOf.get(key))
			.filter((id): id is string => Boolean(id));
		const conversationID = typeof asked.conversationID === 'string' ? asked.conversationID : '';
		const delivered = await tellEach(
			client,
			recipientMemberIDs,
			askedCategory(asked.category),
			{
				...askedNotification(asked),
				senderID: sender ?? '',
				icon: await pictureURLOfSender(client, companyID, asked.senderPicturePath, sender ?? '')
			},
			pushKeys,
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
	pushKeys: PushKeys,
	conversationID: string
): Promise<{ told: number; reached: number; pruned: number }> {
	const nowInSeconds = Math.floor(Date.now() / 1000);
	const deliveries: Delivery[] = [];
	for (const memberID of memberIDs) {
		deliveries.push(await notifyMember(client, memberID, category, notification, pushKeys, nowInSeconds, conversationID));
	}
	return {
		told: deliveries.filter((delivery) => !delivery.silent).length,
		reached: deliveries.reduce((total, delivery) => total + delivery.reached, 0),
		pruned: deliveries.reduce((total, delivery) => total + delivery.pruned, 0)
	};
}

function askedCategory(offered: unknown): NotificationCategory {
	const named = notificationCategories.find((category) => category === offered);
	if (!named) refuse(400, 'that is not a category anything notifies about');
	return named;
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
