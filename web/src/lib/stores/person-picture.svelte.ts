import { isSupabaseConfigured, projectURL, supabase } from '$lib/supabase';
import { keepPersonPictureForReading } from '$lib/messenger/messenger-api';
import { fetchMessengerAccounts } from '$lib/messenger/messenger-accounts';
import { accountsHeldBy, fetchMessengerDirectory, type MessengerDirectory } from '$lib/messenger/messenger-directory';
import { assetBucket, readableAddresses } from '$lib/messenger/kept-attachment';
import { messengerCacheKey, onMessengerCacheReset } from '$lib/messenger/cache-scope';
import {
	freshSignedURLOf,
	keptPictureOf,
	readKeptPictures,
	writeKeptAccounts,
	writeKeptPictures,
	type KeptAccounts,
	type KeptPicture
} from './person-picture-cache';
import { keepPictureFiles } from './person-picture-files';
import { restoreKeptPictures } from './person-picture-restore';

export type PersonIdentity = { memberID?: string; email?: string; externalID?: string };

type HostPicture = { email: string; pictureURL?: string };

type PictureAnswer = { externalID: string; address: string; failed: boolean };

// A person's picture is one object in the company's bucket, named by its
// bytes. What is held here is the address the reader signed for, asked after
// once: a failure leaves nothing behind to be believed on the next visit.
class PersonPictureStore {
	private kept: Map<string, KeptPicture> = new Map();
	private keptAccounts = $state<KeptAccounts | null>(null);
	private restoring = $state(true);
	private readableOfExternal = $state<Map<string, string>>(new Map());
	private urlOfEmail = $state<Map<string, string>>(new Map());
	private resolved = $state<MessengerDirectory | null>(null);
	private avatarURLOfExternal = new Map<string, string>();
	private asked = new Set<string>();
	private directory: Promise<MessengerDirectory | null> | null = null;
	private hostDirectory: Promise<void> | null = null;
	private avatarURLs: Promise<void> | null = null;
	private generation = 0;
	private restored: Promise<void>;

	constructor() {
		this.restored = typeof window === 'undefined' ? Promise.resolve() : this.restore();
		onMessengerCacheReset(() => {
			this.generation += 1;
			this.kept = new Map();
			this.keptAccounts = null;
			this.readableOfExternal = new Map();
			this.urlOfEmail = new Map();
			this.resolved = null;
			this.avatarURLOfExternal.clear();
			this.asked.clear();
			this.directory = null;
			this.hostDirectory = null;
			this.avatarURLs = null;
		});
	}

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

	get isRestoring(): boolean {
		return this.restoring;
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
		const generation = this.generation;
		const wanted = [...new Set(externalIDs)].filter((externalID) => externalID !== '');
		if (wanted.length === 0) return;
		await this.restored;
		await this.knownAvatarURLs();
		if (generation !== this.generation) return;
		if (this.kept.size === 0) this.kept = readKeptPictures(messengerCacheKey());
		this.forgetListedWithoutPicture(wanted);
		const unshown = this.drawKeptWithUnchangedAvatar(wanted);

		const stale = wanted.filter((externalID) => this.needsAsking(externalID));
		if (stale.length === 0 && unshown.length === 0) return;
		stale.forEach((externalID) => this.asked.add(externalID));

		const answers = [...unshown, ...(await Promise.all(stale.map((externalID) => this.askAfter(externalID))))];
		if (generation !== this.generation) return;
		const unsigned = answers.filter((answer) => answer.address !== '' && !this.keptSignedURLOf(answer));
		const readable = await this.signedFor(unsigned.map((answer) => answer.address));
		if (generation !== this.generation) return;
		const shown = await keepPictureFiles(this.readableOfAnswers(answers, readable));
		if (generation !== this.generation) return;
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
			next.set(answer.externalID, shown.get(answer.address) || signed);
			if (!reused) {
				this.kept.set(answer.externalID, keptPictureOf(answer.address, signed, this.avatarURLOf(answer.externalID)));
			}
		}
		this.readableOfExternal = next;
		writeKeptPictures(this.kept, messengerCacheKey());
	}

	private async restore(): Promise<void> {
		const generation = this.generation;
		try {
			const restored = await restoreKeptPictures();
			if (!restored || generation !== this.generation) return;
			this.kept = restored.kept;
			this.keptAccounts = restored.accounts;
			this.readableOfExternal = restored.shownOfExternal;
		} catch (failure) {
			console.warn('the pictures kept on this device could not be read back', failure);
		} finally {
			this.restoring = false;
		}
	}

	private readableOfAnswers(answers: PictureAnswer[], readable: Map<string, string>): Map<string, string> {
		const readableOfAddress = new Map<string, string>();
		for (const answer of answers) {
			const signed = this.keptSignedURLOf(answer) || readable.get(answer.address);
			if (signed) readableOfAddress.set(answer.address, signed);
		}
		return readableOfAddress;
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
		writeKeptPictures(this.kept, messengerCacheKey());
	}

	private drawKeptWithUnchangedAvatar(externalIDs: string[]): PictureAnswer[] {
		const unchanged = externalIDs.filter((externalID) => !this.asked.has(externalID) && this.keptAvatarIsCurrent(externalID));
		const unshown: PictureAnswer[] = [];
		for (const externalID of unchanged) {
			const kept = this.kept.get(externalID);
			if (!kept) continue;
			this.asked.add(externalID);
			if (!this.readableOfExternal.has(externalID)) unshown.push({ externalID, address: kept.address, failed: false });
		}
		return unshown;
	}

	private keptAvatarIsCurrent(externalID: string): boolean {
		const avatarURL = this.avatarURLOf(externalID);
		return avatarURL !== '' && this.kept.get(externalID)?.avatarURL === avatarURL;
	}

	private keptSignedURLOf(answer: PictureAnswer): string {
		const kept = this.kept.get(answer.externalID);
		return kept && kept.address === answer.address ? freshSignedURLOf(kept) : '';
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
		const generation = this.generation;
		this.avatarURLs ??= Promise.resolve()
			.then(() => fetchMessengerAccounts())
			.then((accounts) => {
				if (generation !== this.generation) return;
				for (const person of accounts.people) {
					this.avatarURLOfExternal.set(person.externalID, person.avatarURL ?? '');
				}
			})
			.catch(() => {
				if (generation === this.generation) this.avatarURLs = null;
			});
		return this.avatarURLs;
	}

	private accountsOf(person: PersonIdentity): string[] {
		const directory = this.resolved ?? this.keptAccounts;
		const held = directory ? accountsHeldBy(person, directory) : [];
		return person.externalID ? [person.externalID, ...held] : held;
	}

	private knownPeople(): Promise<MessengerDirectory | null> {
		if (!isSupabaseConfigured()) return Promise.resolve(null);
		const generation = this.generation;
		this.directory ??= fetchMessengerDirectory()
			.then((people) => {
				if (generation !== this.generation) return null;
				writeKeptAccounts(people, messengerCacheKey());
				this.resolved = people;
				return people;
			})
			.catch(() => {
				if (generation === this.generation) this.directory = null;
				return null;
			});
		return this.directory;
	}

	// A device host has no central-plane record to resolve accounts through; it
	// serves the messenger's own directory itself, keyed by the address the
	// company knows each person by.
	private rememberHostDirectory(): Promise<void> {
		const generation = this.generation;
		this.hostDirectory ??= fetch('/agent/api/person-pictures', { credentials: 'include' })
			.then((response) => (response.ok ? response.json() : { pictures: [] }))
			.then((document) => {
				if (generation !== this.generation) return;
				const pictures = ((document as { pictures?: HostPicture[] }).pictures ?? []).filter(
					(picture) => picture.pictureURL
				);
				this.urlOfEmail = new Map(
					pictures.map((picture) => [picture.email.trim().toLowerCase(), picture.pictureURL as string])
				);
			})
			.catch(() => {
				if (generation === this.generation) this.hostDirectory = null;
			});
		return this.hostDirectory;
	}
}

export const personPicture = new PersonPictureStore();
