export type Notification = {
	title: string;
	body: string;
	openPath: string;
	tag: string;
	icon?: string;
	senderName?: string;
	senderID?: string;
};

export type NotificationSender = {
	id: string;
	name: string;
	pictureURL: string;
};

export function senderOf(notification: Notification): NotificationSender | null {
	const pictureURL = notification.icon?.startsWith('https://') ? notification.icon : '';
	if (!pictureURL) return null;
	return {
		id: notification.senderID?.trim() ?? '',
		name: notification.senderName?.trim() || notification.title,
		pictureURL
	};
}

export type PushOutcome = 'delivered' | 'gone' | 'refused';
