import { isSupabaseConfigured } from '$lib/supabase';
import { fetchCustomEmojiImage, fetchCustomEmojiNames } from '$lib/messenger/messenger-api';
import { onMessengerCacheReset } from '$lib/messenger/cache-scope';

type CustomEmojiRecord = { name: string; url: string };

class CustomEmojiStore {
	nameToURL = $state<Map<string, string>>(new Map());
	private named = $state<Set<string>>(new Set());
	private undrawable = $state<Set<string>>(new Set());
	private beingDrawn = new Map<string, Promise<string | null>>();
	private hasLoaded = $state(false);
	private hasFailedToLoad = $state(false);
	private loading: Promise<void> | null = null;
	private generation = 0;

	constructor() {
		onMessengerCacheReset(() => {
			this.generation += 1;
			this.nameToURL = new Map();
			this.named = new Set();
			this.undrawable = new Set();
			this.beingDrawn.clear();
			this.hasLoaded = false;
			this.hasFailedToLoad = false;
			this.loading = null;
		});
	}

	isDrawing(name: string): boolean {
		if (this.nameToURL.has(name) || this.hasFailedToLoad) return false;
		if (!this.hasLoaded) return true;
		return this.named.has(name) && !this.undrawable.has(name);
	}

	load(): Promise<void> {
		if (this.hasLoaded) return Promise.resolve();
		if (!this.loading) {
			const attempt = this.loadOnce().finally(() => { if (this.loading === attempt) this.loading = null; });
			this.loading = attempt;
		}
		return this.loading;
	}

	private async loadOnce(): Promise<void> {
		const generation = this.generation;
		this.hasFailedToLoad = false;
		try {
			if (isSupabaseConfigured()) {
				const names = await fetchCustomEmojiNames();
				if (generation !== this.generation) return;
				this.named = new Set(names);
				this.hasLoaded = true;
				return;
			}
			const response = await fetch('/agent/api/custom-emoji', { credentials: 'include' });
			if (!response.ok) {
				this.hasLoaded = false;
				this.hasFailedToLoad = true;
				return;
			}
			const document: { emoji?: CustomEmojiRecord[] } = await response.json();
			if (generation !== this.generation) return;
			const drawn = document.emoji ?? [];
			this.named = new Set(drawn.map((record) => record.name));
			this.nameToURL = new Map(drawn.map((record) => [record.name, record.url]));
			this.hasLoaded = true;
		} catch {
			if (generation !== this.generation) return;
			this.hasLoaded = false;
			this.hasFailedToLoad = true;
		}
	}

	async draw(names: Iterable<string>): Promise<void> {
		const generation = this.generation;
		const wanted = [...new Set(names)].filter(
			(name) => this.named.has(name) && !this.nameToURL.has(name)
		);
		if (wanted.length === 0) return;
		const drawn = await Promise.all(
			wanted.map(async (name) => [name, await this.drawOnce(name)] as const)
		);
		if (generation !== this.generation) return;
		const filled = new Map(this.nameToURL);
		const undrawable = new Set(this.undrawable);
		for (const [name, url] of drawn) {
			if (url) filled.set(name, url);
			else undrawable.add(name);
		}
		this.nameToURL = filled;
		this.undrawable = undrawable;
	}

	private drawOnce(name: string): Promise<string | null> {
		const already = this.beingDrawn.get(name);
		if (already) return already;
		const drawing = fetchCustomEmojiImage(name)
			.then((image) => image?.dataURL ?? null)
			.catch(() => null)
			.then((url) => {
				if (!url && this.beingDrawn.get(name) === drawing) this.beingDrawn.delete(name);
				return url;
			});
		this.beingDrawn.set(name, drawing);
		return drawing;
	}
}

export const customEmoji = new CustomEmojiStore();
