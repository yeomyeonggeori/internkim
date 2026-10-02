export function taskAccountScope(member: { companyID: string; memberID: string; role: string }, plane: string): string {
	return JSON.stringify([plane, member.companyID, member.memberID, member.role]);
}
