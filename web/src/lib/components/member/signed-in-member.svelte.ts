import { isSupabaseConfigured, supabaseMember } from '$lib/supabase-session';

class SignedInMemberID {
	#memberID = $state('');
	#isRequested = false;

	is(memberID: string): boolean {
		this.#request();
		return memberID !== '' && memberID === this.#memberID;
	}

	#request(): void {
		if (this.#isRequested || !isSupabaseConfigured()) return;
		this.#isRequested = true;
		supabaseMember().then(
			(member) => (this.#memberID = member.memberID),
			() => (this.#isRequested = false)
		);
	}
}

export const signedInMemberID = new SignedInMemberID();
