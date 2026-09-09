import { RecordRefusedTheWrite, statusOfPostgresCode } from './tasks';
import {
	categoriesChoosableBy,
	choicesFromStored,
	isNotificationCategory,
	storedFromChoices,
	type NotificationChoices
} from './notifications';
import type { RecordContext } from './company';
import { notificationCategories, type NotificationCategory } from '../catalog/notifications';
import type {
	ConversationMuteResult,
	NotificationSettingsResult,
	PushReachabilityResult
} from '../catalog/notifications';

export type NotificationSettingsSetInput = { turnOn?: string[]; turnOff?: string[] };

export type ConversationMuteInput = { conversationID?: string };

export type PushDeviceClaimInput = {
	endpoint?: string;
	kind?: string;
	publicKey?: string;
	authenticationSecret?: string;
};

export type PushDeviceReleaseInput = { endpoint?: string; kind?: string };

export type PushDeviceKind = 'web-push' | 'apns' | 'fcm';

export const pushDeviceKinds: readonly PushDeviceKind[] = ['web-push', 'apns', 'fcm'];

function kindAsked(carried: string | undefined): PushDeviceKind {
	const named = carried?.trim() ?? '';
	if (named === '' || named === 'web-push') return 'web-push';
	if (named === 'apns') return 'apns';
	if (named === 'fcm') return 'fcm';
	throw new Error(`a device is reached by one of ${pushDeviceKinds.join(', ')}`);
}

function requesterAdministers(context: RecordContext): boolean {
	return context.people.find((person) => person.personID === context.requesterID)?.isAdmin === true;
}

async function storedChoices(context: RecordContext): Promise<NotificationChoices> {
	const { data, error } = await context.caller.rpc('my_notification_settings');
	if (error) throw new Error(error.message);
	return choicesFromStored(data);
}

async function mutedConversationIDs(context: RecordContext): Promise<string[]> {
	const { data, error } = await context.caller
		.from('notification')
		.select('conversation_id')
		.eq('is_muted', true)
		.returns<{ conversation_id: string }[]>();
	if (error) throw new Error(error.message);
	return (data ?? []).map((row) => row.conversation_id).sort();
}

async function settingsAnswered(context: RecordContext): Promise<NotificationSettingsResult> {
	const chosen = await storedChoices(context);
	const choosable = categoriesChoosableBy(requesterAdministers(context));
	return {
		categories: notificationCategories.map((category) => ({
			category,
			isOn: chosen[category],
			isChoosable: choosable.includes(category)
		})),
		mutedConversationIDs: await mutedConversationIDs(context)
	};
}

export async function notificationSettingsGet(
	context: RecordContext
): Promise<NotificationSettingsResult> {
	return settingsAnswered(context);
}

function categoryAsked(named: string, choosable: NotificationCategory[]): NotificationCategory {
	const category = named.trim();
	if (!isNotificationCategory(category)) {
		throw new RecordRefusedTheWrite(
			`nobody is told about ${category || 'a category with no name'}, so there is nothing to choose`,
			400,
			'no_such_notification_category'
		);
	}
	if (!choosable.includes(category)) {
		throw new RecordRefusedTheWrite(
			`only an administrator chooses whether to be told about ${category}`,
			403,
			'category_not_choosable'
		);
	}
	return category;
}

export async function notificationSettingsSet(
	context: RecordContext,
	input: NotificationSettingsSetInput
): Promise<NotificationSettingsResult> {
	const turnOn = input.turnOn ?? [];
	const turnOff = input.turnOff ?? [];
	if (turnOn.length === 0 && turnOff.length === 0) {
		throw new Error('a notification change names at least one category to turn on or off');
	}

	const choosable = categoriesChoosableBy(requesterAdministers(context));
	const chosen = await storedChoices(context);
	for (const named of turnOn) chosen[categoryAsked(named, choosable)] = true;
	for (const named of turnOff) chosen[categoryAsked(named, choosable)] = false;

	const { error } = await context.caller.rpc('notification_settings_set', {
		chosen: storedFromChoices(chosen)
	});
	if (error) throw new RecordRefusedTheWrite(error.message, statusOfPostgresCode(error.code));
	return settingsAnswered(context);
}

function conversationAsked(input: ConversationMuteInput): string {
	const conversationID = input.conversationID?.trim();
	if (!conversationID) throw new Error('this call names the conversation it is about');
	return conversationID;
}

async function mutingAnswered(
	context: RecordContext,
	conversationID: string,
	isMuted: boolean
): Promise<ConversationMuteResult> {
	return { conversationID, isMuted, mutedConversationIDs: await mutedConversationIDs(context) };
}

export async function conversationMute(
	context: RecordContext,
	input: ConversationMuteInput
): Promise<ConversationMuteResult> {
	const conversationID = conversationAsked(input);
	const { error } = await context.caller.rpc('conversation_mute', { conversation: conversationID });
	if (error) throw new RecordRefusedTheWrite(error.message, statusOfPostgresCode(error.code));
	return mutingAnswered(context, conversationID, true);
}

export async function conversationUnmute(
	context: RecordContext,
	input: ConversationMuteInput
): Promise<ConversationMuteResult> {
	const conversationID = conversationAsked(input);
	const { error } = await context.caller.rpc('conversation_unmute', { conversation: conversationID });
	if (error) throw new RecordRefusedTheWrite(error.message, statusOfPostgresCode(error.code));
	return mutingAnswered(context, conversationID, false);
}

async function vaultedServerKey(context: RecordContext): Promise<string> {
	const { data, error } = await context.caller.rpc('vapid_public_key');
	if (error) throw new Error(error.message);
	return typeof data === 'string' ? data : '';
}

async function hasClaimedDevice(context: RecordContext): Promise<boolean> {
	const { data, error } = await context.caller
		.from('push_device')
		.select('address')
		.limit(1)
		.returns<{ address: string }[]>();
	if (error) throw new Error(error.message);
	return (data ?? []).length > 0;
}

async function reachabilityAnswered(context: RecordContext): Promise<PushReachabilityResult> {
	const serverKey = await vaultedServerKey(context);
	return {
		serverKey,
		isServerKeyVaulted: serverKey !== '',
		hasClaimedDevice: await hasClaimedDevice(context)
	};
}

export async function pushReachabilityGet(context: RecordContext): Promise<PushReachabilityResult> {
	return reachabilityAnswered(context);
}

function endpointAsked(input: { endpoint?: string }): string {
	const endpoint = input.endpoint?.trim();
	if (!endpoint) throw new Error('this call names the subscription it is about');
	return endpoint;
}

function encryptionKeysAsked(input: PushDeviceClaimInput): { p256dh: string; auth: string } {
	const publicKey = input.publicKey?.trim();
	const authenticationSecret = input.authenticationSecret?.trim();
	if (!publicKey || !authenticationSecret) {
		throw new Error('a claimed subscription carries both of the keys push is encrypted to');
	}
	return { p256dh: publicKey, auth: authenticationSecret };
}

export async function pushDeviceClaim(
	context: RecordContext,
	input: PushDeviceClaimInput
): Promise<PushReachabilityResult> {
	const kind = kindAsked(input.kind);
	const { error } = await context.caller.rpc('push_device_claim', {
		device_kind: kind,
		device_address: endpointAsked(input),
		device_keys: kind === 'web-push' ? encryptionKeysAsked(input) : {}
	});
	if (error) throw new RecordRefusedTheWrite(error.message, statusOfPostgresCode(error.code));
	return reachabilityAnswered(context);
}

export async function pushDeviceRelease(
	context: RecordContext,
	input: PushDeviceReleaseInput
): Promise<PushReachabilityResult> {
	const { error } = await context.caller.rpc('push_device_release', {
		device_kind: kindAsked(input.kind),
		device_address: endpointAsked(input)
	});
	if (error) throw new RecordRefusedTheWrite(error.message, statusOfPostgresCode(error.code));
	return reachabilityAnswered(context);
}
