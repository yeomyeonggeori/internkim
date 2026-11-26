import type { SupabaseClient } from './service-client.ts';
import { readNotificationSettings, type NotificationSettings } from './categories.ts';
import type { Placed } from './colleague-reach.ts';

export type CompanyColleague = Placed & { notificationSettings: NotificationSettings };

type ColleagueRow = { id: string; team_id: string | null; supervisor_id: string | null; preferences: unknown };

export async function colleaguesOfCompany(record: SupabaseClient, companyID: string): Promise<CompanyColleague[]> {
	const { data, error } = await record
		.from('member')
		.select('id, team_id, supervisor_id, preferences')
		.eq('company_id', companyID)
		.eq('can_read_the_company', true)
		.returns<ColleagueRow[]>();
	if (error) throw new Error(error.message);
	return (data ?? []).map((member) => ({
		id: member.id,
		teamID: member.team_id,
		supervisorID: member.supervisor_id,
		notificationSettings: readNotificationSettings(notificationSettingsOf(member.preferences))
	}));
}

function notificationSettingsOf(preferences: unknown): unknown {
	return typeof preferences === 'object' && preferences !== null
		? Reflect.get(preferences, 'notificationSettings')
		: undefined;
}
