import { isSupabaseConfigured } from '$lib/supabase';
import { fetchProfilePicture } from '$lib/messenger/messenger-api';
import { accountsHeldBy, fetchMessengerDirectory, type MessengerDirectory } from '$lib/messenger/messenger-directory';

export type PersonIdentity = { memberID?: string; email?: string };

type HostPicture = { email: string; pictureURL?: string };

class PersonPictureStore {
	private dataURLOfExternal = $state<Map<string, string>>(new Map());
	private urlOfEmail = $state<Map<string, string>>(new Map());
	private resolved = $state<MessengerDirectory | null>(null);
	private directory: Promise<MessengerDirectory | null> | null = null;
	private hostDirectory: Promise<void> | null = null;
	private asked = new Set<string>();

	// Somebody who was on one messenger and is now on another holds an account on
	// each, and only the one the company runs today has a picture to give. Which
	// that is belongs to the host, so it is not guessed here: every account the
	// person holds is asked after, and the one that answers is their picture.
	pictureOf(person: PersonIdentity): string {
		const byEmail = this.urlOfEmail.get((person.email ?? '').trim().toLowerCase());
		if (byEmail) return byEmail;
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
		if (!isSupabaseConfigured()) return this.rememberHostDirectory();
		await this.knownPeople();
		await this.rememberExternals(people.flatMap((person) => this.accountsOf(person)));
	}

	async rememberExternals(externalIDs: string[]): Promise<void> {
		const wanted = [...new Set(externalIDs)].filter((externalID) => externalID && !this.asked.has(externalID));
		if (wanted.length === 0) return;
		wanted.forEach((externalID) => this.asked.add(externalID));

		const answers = await Promise.all(wanted.map((externalID) => this.askAfter(externalID)));
		const next = new Map(this.dataURLOfExternal);
		for (const answer of answers) {
			if (answer.failed) this.asked.delete(answer.externalID);
			else next.set(answer.externalID, answer.dataURL);
		}
		this.dataURLOfExternal = next;
	}

	private async askAfter(externalID: string): Promise<{ externalID: string; dataURL: string; failed: boolean }> {
		try {
			const picture = await fetchProfilePicture(externalID);
			return { externalID, dataURL: picture?.dataURL ?? '', failed: false };
		} catch {
			return { externalID, dataURL: '', failed: true };
		}
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

	// A device host has no central-plane record to resolve accounts through; it
	// serves the messenger's own directory itself, keyed by the address the
	// company knows each person by.
	private rememberHostDirectory(): Promise<void> {
		this.hostDirectory ??= fetch('/agent/api/person-pictures', { credentials: 'include' })
			.then((response) => (response.ok ? response.json() : { pictures: [] }))
			.then((document) => {
				const pictures = ((document as { pictures?: HostPicture[] }).pictures ?? []).filter(
					(picture) => picture.pictureURL
				);
				this.urlOfEmail = new Map(
					pictures.map((picture) => [picture.email.trim().toLowerCase(), picture.pictureURL as string])
				);
			})
			.catch(() => {
				this.hostDirectory = null;
			});
		return this.hostDirectory;
	}
}

export const personPicture = new PersonPictureStore();
