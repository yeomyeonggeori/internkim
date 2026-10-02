import { isSupabaseConfigured, projectURL, supabase } from '$lib/supabase';
import { fetchPeople, keepPersonPictureForReading } from '$lib/messenger/messenger-api';
import { accountsHeldBy, fetchMessengerDirectory, type MessengerDirectory } from '$lib/messenger/messenger-directory';
import { assetBucket, readableAddresses } from '$lib/messenger/kept-attachment';
import { keptPictureOf, readKeptPictures, writeKeptPictures, type KeptPicture } from './person-picture-cache';

export type PersonIdentity = { memberID?: string; email?: string; externalID?: string };

type HostPicture = { email: string; pictureURL?: string };

type PictureAnswer = { externalID: string; address: string; failed: boolean };

// A person's picture is one object in the company's bucket, named by its
// bytes. What is held here is the address the reader signed for, asked after
// once: a failure leaves nothing behind to be believed on the next visit.
class PersonPictureStore {
	private kept: Map<string, KeptPicture> = readKeptPictures();
	private readableOfExternal = $state<Map<string, string>>(
		new Map([...this.kept].map(([externalID, picture]) => [externalID, picture.signedURL]))
	);
	private urlOfEmail = $state<Map<string, string>>(new Map());
	private resolved = $state<MessengerDirectory | null>(null);
	private avatarURLOfExternal = new Map<string, string>();
	private asked = new Set<string>();
	private directory: Promise<MessengerDirectory | null> | null = null;
	private hostDirectory: Promise<void> | null = null;
	private avatarURLs: Promise<void> | null = null;

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
		return this.readableOfExternal.get(externalID) ?? '';
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
		if (!isSupabaseConfigured()) return this.rememberHostDirectory();
		const wanted = [...new Set(externalIDs)].filter((externalID) => externalID !== '');
		if (wanted.length === 0) return;
		await this.knownAvatarURLs();
		this.forgetListedWithoutPicture(wanted);

		const stale = wanted.filter((externalID) => this.needsAsking(externalID));
		if (stale.length === 0) return;
		stale.forEach((externalID) => this.asked.add(externalID));

		const answers = await Promise.all(stale.map((externalID) => this.askAfter(externalID)));
		const unsigned = answers.filter((answer) => answer.address !== '' && !this.keptSignedURLOf(answer));
		const readable = await this.signedFor(unsigned.map((answer) => answer.address));
		const next = new Map(this.readableOfExternal);
		for (const answer of answers) {
			if (answer.address === '' && !answer.failed) {
				next.delete(answer.externalID);
				this.kept.delete(answer.externalID);
				continue;
			}
			const reused = this.keptSignedURLOf(answer);
			const signed = reused || readable.get(answer.address);
			if (!signed) {
				this.asked.delete(answer.externalID);
				continue;
			}
			next.set(answer.externalID, signed);
			if (!reused) this.kept.set(answer.externalID, keptPictureOf(answer.address, signed));
		}
		this.readableOfExternal = next;
		writeKeptPictures(this.kept);
	}

	private forgetListedWithoutPicture(externalIDs: string[]): void {
		const gone = externalIDs.filter((externalID) => this.isListedWithoutPicture(externalID) && this.readableOfExternal.has(externalID));
		if (gone.length === 0) return;
		const next = new Map(this.readableOfExternal);
		for (const externalID of gone) {
			next.delete(externalID);
			this.kept.delete(externalID);
		}
		this.readableOfExternal = next;
		writeKeptPictures(this.kept);
	}

	private keptSignedURLOf(answer: PictureAnswer): string {
		const kept = this.kept.get(answer.externalID);
		return kept && kept.address === answer.address ? kept.signedURL : '';
	}

	private avatarURLOf(externalID: string): string {
		return this.avatarURLOfExternal.get(externalID) ?? '';
	}

	private needsAsking(externalID: string): boolean {
		return !this.isListedWithoutPicture(externalID) && !this.asked.has(externalID);
	}

	private isListedWithoutPicture(externalID: string): boolean {
		return this.avatarURLOfExternal.get(externalID) === '';
	}

	private async askAfter(externalID: string): Promise<PictureAnswer> {
		try {
			const kept = await keepPersonPictureForReading({ externalID, avatarURL: this.avatarURLOf(externalID) });
			return { externalID, address: kept?.address ?? '', failed: false };
		} catch {
			return { externalID, address: '', failed: true };
		}
	}

	private signedFor(addresses: string[]): Promise<Map<string, string>> {
		if (addresses.length === 0) return Promise.resolve(new Map());
		return readableAddresses(supabase().storage.from(assetBucket), projectURL(), addresses);
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
