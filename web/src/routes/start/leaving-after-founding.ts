// The founded card carries each invited person's temporary password, and it is
// the only time any of them is readable. Nothing may navigate past it.
export function leavesRightAfterFounding(returnPath: string, invitationCount: number): boolean {
	return returnPath !== '' && invitationCount === 0;
}
