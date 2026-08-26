import { supabase } from '$lib/supabase';
import { readNotificationSettings, type NotificationSettings } from './categories';

export async function myNotificationSettings(): Promise<NotificationSettings> {
	const { data } = await supabase().auth.getSession();
	const accountID = data.session?.user.id;
	if (!accountID) return readNotificationSettings({});

	const member = await supabase()
		.from('member')
		.select('notification_settings')
		.eq('user_id', accountID)
		.maybeSingle<{ notification_settings: unknown }>();
	if (member.error) throw new Error(member.error.message);
	return readNotificationSettings(member.data?.notification_settings);
}

export async function chooseNotificationSettings(chosen: NotificationSettings): Promise<void> {
	const { error } = await supabase().rpc('notification_settings_set', { chosen });
	if (error) throw new Error(error.message);
}
