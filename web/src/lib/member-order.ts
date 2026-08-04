export type OrderableMember = {
	id: string;
	name: string | null;
	email: string | null;
	joined_at: string | null;
};

// Everyone looks for themselves first, then reads down the list the way the
// company grew: who joined earliest, and by name when they joined the same day
// or never recorded one.
export function membersInReadingOrder<Member extends OrderableMember>(
	members: Member[],
	readerID: string | undefined,
): Member[] {
	return [...members].sort((left, right) => {
		if (left.id === readerID) return -1;
		if (right.id === readerID) return 1;
		const byJoining = joiningRank(left).localeCompare(joiningRank(right));
		if (byJoining !== 0) return byJoining;
		return displayNameOf(left).localeCompare(displayNameOf(right), 'ko');
	});
}

// Someone with no recorded joining day sorts after everyone who has one, rather
// than claiming the earliest place by being empty.
function joiningRank(member: OrderableMember): string {
	return member.joined_at ? member.joined_at.slice(0, 10) : '9999-12-31';
}

function displayNameOf(member: OrderableMember): string {
	return member.name || (member.email ?? '').split('@')[0];
}
