import { RecordRefusedTheWrite, statusOfPostgresCode } from './tasks';
import {
	attendanceReachFromStored,
	categoriesChoosableBy,
	choicesFromStored,
	isAttendanceReach,
	isNotificationCategory,
	storedFromChoices
} from './notifications';
import type { RecordContext } from './company';
import {
	notificationCategories,
	notificationSettingsResultSchema,
	type AttendanceReach,
	type NotificationCategory
} from '../catalog/notifications';
import type { ConversationMuteResult, NotificationSettingsResult } from '../catalog/notifications';

export type NotificationSettingsSetInput = { turnOn?: string[]; turnOff?: string[]; attendanceReach?: string };

export type ConversationMuteInput = { conversationID?: string };

function requesterAdministers(context: RecordContext): boolean {
	return context.people.find((person) => person.personID === context.requesterID)?.isAdmin === true;
}

async function storedSettings(context: RecordContext): Promise<unknown> {
	const { data, error } = await context.caller.rpc('my_notification_settings');
	if (error) throw new Error(error.message);
	return data;
}

async function mutedConversationIDs(context: RecordContext): Promise<string[]> {
	const { data, error } = await context.caller.rpc('my_muted_conversations');
	if (error) throw new Error(error.message);
	return notificationSettingsResultSchema.shape.mutedConversationIDs.parse(data).sort();
}

async function settingsAnswered(context: RecordContext): Promise<NotificationSettingsResult> {
	const stored = await storedSettings(context);
	const chosen = choicesFromStored(stored);
	const choosable = categoriesChoosableBy(requesterAdministers(context));
	return {
		categories: notificationCategories.map((category) => ({
			category,
			isOn: chosen[category],
			isChoosable: choosable.includes(category)
		})),
		attendanceReach: attendanceReachFromStored(stored),
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

function reachAsked(named: string): AttendanceReach {
	const reach = named.trim();
	if (!isAttendanceReach(reach)) {
		throw new RecordRefusedTheWrite(
			`${reach || 'a reach with no name'} is not a reach attendance notices have`,
			400,
			'no_such_attendance_reach'
		);
	}
	return reach;
}

export async function notificationSettingsSet(
	context: RecordContext,
	input: NotificationSettingsSetInput
): Promise<NotificationSettingsResult> {
	const turnOn = input.turnOn ?? [];
	const turnOff = input.turnOff ?? [];
	if (turnOn.length === 0 && turnOff.length === 0 && input.attendanceReach === undefined) {
		throw new Error('a notification change names at least one category to turn on or off, or an attendance reach');
	}

	const choosable = categoriesChoosableBy(requesterAdministers(context));
	const stored = await storedSettings(context);
	const chosen = choicesFromStored(stored);
	for (const named of turnOn) chosen[categoryAsked(named, choosable)] = true;
	for (const named of turnOff) chosen[categoryAsked(named, choosable)] = false;
	const attendanceReach =
		input.attendanceReach === undefined ? attendanceReachFromStored(stored) : reachAsked(input.attendanceReach);

	const { error } = await context.caller.rpc('notification_settings_set', {
		chosen: storedFromChoices(chosen, attendanceReach)
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
