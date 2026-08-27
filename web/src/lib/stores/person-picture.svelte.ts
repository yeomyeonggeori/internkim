import { isSupabaseConfigured } from '$lib/supabase';
import { fetchProfilePicture } from '$lib/messenger/messenger-api';
import { accountsHeldBy, fetchMessengerDirectory, type MessengerDirectory } from '$lib/messenger/messenger-directory';

export type PersonIdentity = { memberID?: string; email?: string };

class PersonPictureStore {
	private dataURLOfExternal = $state<Map<string, string>>(new Map());
	private resolved = $state<MessengerDirectory | null>(null);
	private directory: Promise<MessengerDirectory | null> | null = null;
	private asked = new Set<string>();

	// Somebody who was on one messenger and is now on another holds an account on
	// each, and only the one the company runs today has a picture to give. Which
	// that is belongs to the host, so it is not guessed here: every account the
	// person holds is asked after, and the one that answers is their picture.
	pictureOf(person: PersonIdentity): string {
		for (const externalID of this.accountsOf(person)) {
			const drawn = this.pictureOfExternal(externalID);
			if (drawn) return drawn;
		}
		return '';
	}

	pictureOfExternal(externalID: string): string {
		return this.dataURLOfExternal.get(externalID) ?? '';
	}

	async remember(people: PersonIdentity[]): Promise<void> {
		await this.knownPeople();
		await this.rememberExternals(people.flatMap((person) => this.accountsOf(person)));
	}

	async rememberExternals(externalIDs: string[]): Promise<void> {
		const wanted = [...new Set(externalIDs)].filter((externalID) => externalID && !this.asked.has(externalID));
		if (wanted.length === 0) return;
		wanted.forEach((externalID) => this.asked.add(externalID));

		const drawn = await Promise.all(
			wanted.map(async (externalID) => {
				const picture = await fetchProfilePicture(externalID).catch(() => null);
				return [externalID, picture?.dataURL ?? ''] as const;
			})
		);
		const next = new Map(this.dataURLOfExternal);
		for (const [externalID, dataURL] of drawn) next.set(externalID, dataURL);
		this.dataURLOfExternal = next;
	}

	private accountsOf(person: PersonIdentity): string[] {
		return this.resolved ? accountsHeldBy(person, this.resolved) : [];
	}

	private knownPeople(): Promise<MessengerDirectory | null> {
		if (!isSupabaseConfigured()) return Promise.resolve(null);
		this.directory ??= fetchMessengerDirectory()
			.then((people) => (this.resolved = people))
			.catch(() => {
				this.directory = null;
				return null;
			});
		return this.directory;
	}
}

export const personPicture = new PersonPictureStore();
