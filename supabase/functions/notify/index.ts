import { callingAgent } from '../_shared/agent-caller.ts';
import { notificationCategories, type NotificationCategory } from '../_shared/categories.ts';
import { askedObject, json, refuse, serveRefusals } from '../_shared/http.ts';
import { membersOfCompanyByEmail, membersOfCompanyByExternalID, signedPictureURL } from '../_shared/member-directory.ts';
import { addressedIn } from '../_shared/notify-recipients.ts';
import { bodyWithAttachmentLabels } from '../_shared/attachment-labels.ts';
import { localeOfMember } from '../_shared/member-locale.ts';
import { isCompanySharedPath, pictureURLOfSender } from '../_shared/sender-picture.ts';
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
	messageID?: unknown;
	withdrawnMessageIDs?: unknown;
	senderExternalID?: unknown;
	senderPicturePath?: unknown;
	attachmentKinds?: unknown;
	messagePicturePath?: unknown;
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
				icon: await pictureURLOfSender(client, companyID, asked.senderPicturePath, sender ?? ''),
				...(await messagePictureOf(client, companyID, asked.messagePicturePath))
			},
			pushKeys,
			conversationID,
			askedAttachmentKinds(asked)
		);

		return json({ ...delivered, addressed: recipients.length });
	})
);

async function tellEach(
	client: SupabaseClient,
	memberIDs: string[],
	category: NotificationCategory,
	notification: Notification,
	pushKeys: PushKeys,
	conversationID: string,
	attachmentKinds: string[]
): Promise<{ told: number; reached: number; pruned: number }> {
	const nowInSeconds = Math.floor(Date.now() / 1000);
	const deliveries: Delivery[] = [];
	for (const memberID of memberIDs) {
		const told = await notificationFor(client, memberID, notification, attachmentKinds);
		deliveries.push(await notifyMember(client, memberID, category, told, pushKeys, nowInSeconds, conversationID));
	}
	return {
		told: deliveries.filter((delivery) => !delivery.silent).length,
		reached: deliveries.reduce((total, delivery) => total + delivery.reached, 0),
		pruned: deliveries.reduce((total, delivery) => total + delivery.pruned, 0)
	};
}

async function notificationFor(
	client: SupabaseClient,
	memberID: string,
	notification: Notification,
	attachmentKinds: string[]
): Promise<Notification> {
	if (attachmentKinds.length === 0) return notification;
	const locale = await localeOfMember(client, memberID);
	return { ...notification, body: bodyWithAttachmentLabels(notification.body, attachmentKinds, locale) };
}

async function messagePictureOf(
	client: SupabaseClient,
	companyID: string,
	path: unknown
): Promise<Pick<Notification, 'imageURL'>> {
	if (!isCompanySharedPath(companyID, path)) return {};
	const imageURL = await signedPictureURL(client, path);
	return imageURL ? { imageURL } : {};
}

function askedAttachmentKinds(asked: NotifyRequest): string[] {
	if (!Array.isArray(asked.attachmentKinds)) return [];
	return asked.attachmentKinds.filter((kind): kind is string => typeof kind === 'string');
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
		tag: typeof asked.tag === 'string' ? asked.tag : 'internkim',
		messageID: typeof asked.messageID === 'string' ? asked.messageID : '',
		withdrawnMessageIDs: Array.isArray(asked.withdrawnMessageIDs)
			? asked.withdrawnMessageIDs.filter((messageID): messageID is string => typeof messageID === 'string' && messageID !== '')
			: []
	};
}
