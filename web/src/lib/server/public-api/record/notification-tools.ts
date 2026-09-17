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
import type { ConversationMuteResult, NotificationSettingsResult } from '../catalog/notifications';

export type NotificationSettingsSetInput = { turnOn?: string[]; turnOff?: string[] };

export type ConversationMuteInput = { conversationID?: string };

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
