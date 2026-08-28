import { supabase } from '$lib/supabase';
import { readNotificationSettings, writeNotificationSettings, type NotificationSettings } from './categories';

export async function myNotificationSettings(): Promise<NotificationSettings> {
	const { data } = await supabase().auth.getSession();
	const accountID = data.session?.user.id;
	if (!accountID) return readNotificationSettings({});

	const settings = await supabase().rpc('my_notification_settings');
	if (settings.error) throw new Error(settings.error.message);
	return readNotificationSettings(settings.data);
}

export async function chooseNotificationSettings(chosen: NotificationSettings): Promise<void> {
	const { error } = await supabase().rpc('notification_settings_set', { chosen: writeNotificationSettings(chosen) });
	if (error) throw new Error(error.message);
}
