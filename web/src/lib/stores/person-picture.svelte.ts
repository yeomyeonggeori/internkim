import { isSupabaseConfigured } from '$lib/supabase';
import { fetchProfilePicture } from '$lib/messenger/messenger-api';
import { externalIDFor, fetchMessengerDirectory, type MessengerDirectory } from '$lib/messenger/messenger-directory';

export type PersonIdentity = { memberID?: string; email?: string };

class PersonPictureStore {
	private dataURLOfExternal = $state<Map<string, string>>(new Map());
	private resolved = $state<MessengerDirectory | null>(null);
	private directory: Promise<MessengerDirectory | null> | null = null;
	private asked = new Set<string>();

	pictureOf(person: PersonIdentity): string {
		return this.pictureOfExternal(this.externalIDOf(person));
	}

	pictureOfExternal(externalID: string): string {
		return this.dataURLOfExternal.get(externalID) ?? '';
	}

	async remember(people: PersonIdentity[]): Promise<void> {
		await this.knownPeople();
		await this.rememberExternals(people.map((person) => this.externalIDOf(person)));
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

	private externalIDOf(person: PersonIdentity): string {
		return this.resolved ? externalIDFor(person, this.resolved) : '';
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
