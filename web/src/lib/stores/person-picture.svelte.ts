import { isSupabaseConfigured } from '$lib/supabase';
import { fetchPeople, fetchProfilePicture } from '$lib/messenger/messenger-api';
import { accountsHeldBy, fetchMessengerDirectory, type MessengerDirectory } from '$lib/messenger/messenger-directory';
import { forgetCachedPicture, readCachedPictures, writeCachedPicture } from '$lib/person-picture-cache';

export type PersonIdentity = { memberID?: string; email?: string; externalID?: string };

type HostPicture = { email: string; pictureURL?: string };

// buzz-relay serves a picture from /media/<sha256 of its bytes> (Blossom
// BUD-02), so an avatar URL never holds different bytes later and a kept copy
// is current exactly while the URL it was kept from is the one the person
// still carries.
class PersonPictureStore {
	private dataURLOfExternal = $state<Map<string, string>>(new Map());
	private urlOfEmail = $state<Map<string, string>>(new Map());
	private resolved = $state<MessengerDirectory | null>(null);
	private keptFromAvatarURL = new Map<string, string>();
	private avatarURLOfExternal = new Map<string, string>();
	private directory: Promise<MessengerDirectory | null> | null = null;
	private hostDirectory: Promise<void> | null = null;
	private keptPictures: Promise<void> | null = null;
	private avatarURLs: Promise<void> | null = null;
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

	async rememberEveryone(): Promise<void> {
		if (!isSupabaseConfigured()) return this.rememberHostDirectory();
		const people = await this.knownPeople();
		if (!people) return;
		await this.rememberExternals([...people.externalsOfMember.values()].flat());
	}

	async rememberExternals(externalIDs: string[]): Promise<void> {
		const wanted = [...new Set(externalIDs)].filter((externalID) => externalID !== '');
		if (wanted.length === 0) return;
		await this.restoreKeptPictures();
		await this.knownAvatarURLs();
		this.forgetPicturesTakenDown(wanted);

		const stale = wanted.filter((externalID) => this.needsRedrawing(externalID));
		if (stale.length === 0) return;
		stale.forEach((externalID) => this.asked.add(externalID));

		const answers = await Promise.all(stale.map((externalID) => this.askAfter(externalID)));
		const next = new Map(this.dataURLOfExternal);
		for (const answer of answers) {
			if (answer.failed) {
				this.asked.delete(answer.externalID);
				continue;
			}
			const avatarURL = this.avatarURLOfExternal.get(answer.externalID) ?? '';
			next.set(answer.externalID, answer.dataURL);
			this.keptFromAvatarURL.set(answer.externalID, avatarURL);
			void writeCachedPicture({ externalID: answer.externalID, avatarURL, dataURL: answer.dataURL });
		}
		this.dataURLOfExternal = next;
	}

	private needsRedrawing(externalID: string): boolean {
		if (this.asked.has(externalID)) return false;
		const avatarURL = this.avatarURLOfExternal.get(externalID);
		if (avatarURL === '') return false;
		if (avatarURL === undefined) return true;
		return this.keptFromAvatarURL.get(externalID) !== avatarURL;
	}

	private forgetPicturesTakenDown(externalIDs: string[]): void {
		const takenDown = externalIDs.filter(
			(externalID) =>
				this.avatarURLOfExternal.get(externalID) === '' && this.dataURLOfExternal.has(externalID)
		);
		if (takenDown.length === 0) return;
		const next = new Map(this.dataURLOfExternal);
		for (const externalID of takenDown) {
			next.delete(externalID);
			this.keptFromAvatarURL.delete(externalID);
			void forgetCachedPicture(externalID);
		}
		this.dataURLOfExternal = next;
	}

	private restoreKeptPictures(): Promise<void> {
		this.keptPictures ??= readCachedPictures().then((pictures) => {
			if (pictures.length === 0) return;
			const next = new Map(this.dataURLOfExternal);
			for (const picture of pictures) {
				next.set(picture.externalID, picture.dataURL);
				this.keptFromAvatarURL.set(picture.externalID, picture.avatarURL);
			}
			this.dataURLOfExternal = next;
		});
		return this.keptPictures;
	}

	private knownAvatarURLs(): Promise<void> {
		this.avatarURLs ??= Promise.resolve()
			.then(() => fetchPeople())
			.then((people) => {
				for (const person of people) {
					this.avatarURLOfExternal.set(person.externalID, person.avatarURL ?? '');
				}
			})
			.catch(() => {
				this.avatarURLs = null;
			});
		return this.avatarURLs;
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
		const held = this.resolved ? accountsHeldBy(person, this.resolved) : [];
		return person.externalID ? [person.externalID, ...held] : held;
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
