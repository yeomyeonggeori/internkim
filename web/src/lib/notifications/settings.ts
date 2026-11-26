import { invokeTool } from '$lib/public-api-call';
import {
	isAttendanceReach,
	isNotificationCategory,
	isTaskReach,
	type AttendanceReach,
	type NotificationCategory,
	type TaskReach
} from './categories';

export type NotificationChoice = {
	category: NotificationCategory;
	isOn: boolean;
	isChoosable: boolean;
};

export type NotificationSettings = {
	categories: NotificationChoice[];
	attendanceReach: AttendanceReach;
	taskReach: TaskReach;
	mutedConversationIDs: string[];
};

type AnsweredSettings = {
	categories: { category: string; isOn: boolean; isChoosable: boolean }[];
	attendanceReach: string;
	taskReach: string;
	mutedConversationIDs: string[];
};

function attendanceReachOf(answered: string): AttendanceReach {
	if (!isAttendanceReach(answered)) throw new Error(`the settings answered an unknown attendance reach: ${answered}`);
	return answered;
}

function taskReachOf(answered: string): TaskReach {
	if (!isTaskReach(answered)) throw new Error(`the settings answered an unknown task reach: ${answered}`);
	return answered;
}

function settingsOf(answered: AnsweredSettings): NotificationSettings {
	return {
		categories: answered.categories.flatMap((choice) =>
			isNotificationCategory(choice.category) ? [{ ...choice, category: choice.category }] : []
		),
		attendanceReach: attendanceReachOf(answered.attendanceReach),
		taskReach: taskReachOf(answered.taskReach),
		mutedConversationIDs: answered.mutedConversationIDs
	};
}

export async function myNotificationSettings(): Promise<NotificationSettings> {
	return settingsOf(await invokeTool<AnsweredSettings>('notification_settings_get', {}));
}

export async function chooseNotificationCategory(
	category: NotificationCategory,
	wanted: boolean
): Promise<NotificationSettings> {
	const chosen = wanted ? { turnOn: [category] } : { turnOff: [category] };
	return settingsOf(await invokeTool<AnsweredSettings>('notification_settings_set', chosen));
}

export async function chooseAttendanceReach(reach: AttendanceReach): Promise<NotificationSettings> {
	return settingsOf(
		await invokeTool<AnsweredSettings>('notification_settings_set', { turnOn: ['attendance'], attendanceReach: reach })
	);
}

export async function chooseTaskReach(reach: TaskReach): Promise<NotificationSettings> {
	return settingsOf(await invokeTool<AnsweredSettings>('notification_settings_set', { turnOn: ['task'], taskReach: reach }));
}
