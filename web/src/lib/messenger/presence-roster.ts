export type PresenceState = Record<string, ReadonlyArray<Record<string, unknown>>>;

export function onlineMemberIDs(state: PresenceState): ReadonlySet<string> {
	const online = new Set<string>();
	for (const presences of Object.values(state)) {
		for (const presence of presences) {
			const memberID = presence.memberID;
			if (typeof memberID === 'string' && memberID) online.add(memberID);
		}
	}
	return online;
}
