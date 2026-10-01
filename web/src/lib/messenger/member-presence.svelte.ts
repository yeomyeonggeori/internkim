import { shareCompanyPresence } from '$lib/company-channel';
import { onlineMemberIDs } from '$lib/messenger/presence-roster';
import { isSupabaseConfigured, supabaseMember } from '$lib/supabase-session';

class MemberPresence {
	#online = $state<ReadonlySet<string>>(new Set());

	isOnline(memberID: string): boolean {
		return this.#online.has(memberID);
	}

	keepMineShared(): () => void {
		if (!isSupabaseConfigured()) return () => {};
		let isStopped = false;
		let stopSharing = () => {};
		supabaseMember().then(
			({ memberID }) => {
				if (isStopped || !memberID) return;
				stopSharing = shareCompanyPresence({ memberID }, (state) => {
					this.#online = onlineMemberIDs(state);
				});
			},
			(failure: unknown) => console.warn('presence was not shared', failure)
		);
		return () => {
			isStopped = true;
			stopSharing();
			this.#online = new Set();
		};
	}
}

export const memberPresence = new MemberPresence();

export type AvatarPresence = { isOnline?: boolean; presenceLabel?: string };

export function avatarPresenceOf(
	memberID: string | undefined,
	labels: { presenceOnline: string; presenceOffline: string }
): AvatarPresence {
	if (!memberID) return {};
	const isOnline = memberPresence.isOnline(memberID);
	return { isOnline, presenceLabel: isOnline ? labels.presenceOnline : labels.presenceOffline };
}
