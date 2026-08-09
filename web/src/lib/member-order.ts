export type OrderableMember = {
	id: string;
	name: string | null;
	email: string | null;
	joined_at: string | null;
};

export function membersInReadingOrder<Member extends OrderableMember>(
	members: Member[],
	readerID: string | undefined,
): Member[] {
	return [...members].sort((left, right) => {
		if (left.id === readerID) return -1;
		if (right.id === readerID) return 1;
		const leftJoined = joiningRank(left);
		const rightJoined = joiningRank(right);
		if (leftJoined !== rightJoined) return leftJoined < rightJoined ? -1 : 1;
		const leftName = displayNameOf(left);
		const rightName = displayNameOf(right);
		if (leftName === rightName) return 0;
		return leftName < rightName ? -1 : 1;
	});
}

function joiningRank(member: OrderableMember): string {
	return member.joined_at ? member.joined_at.slice(0, 10) : '9999-12-31';
}

function displayNameOf(member: OrderableMember): string {
	return member.name || (member.email ?? '').split('@')[0];
}
