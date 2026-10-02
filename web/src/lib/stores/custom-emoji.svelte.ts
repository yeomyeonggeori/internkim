import { isSupabaseConfigured } from '$lib/supabase';
import { fetchCustomEmojiImage, fetchCustomEmojiNames } from '$lib/messenger/messenger-api';
import { onMessengerCacheReset } from '$lib/messenger/cache-scope';

type CustomEmojiRecord = { name: string; url: string };

class CustomEmojiStore {
	nameToURL = $state<Map<string, string>>(new Map());
	private named = new Set<string>();
	private beingDrawn = new Map<string, Promise<string | null>>();
	private hasLoaded = false;
	private loading: Promise<void> | null = null;
	private generation = 0;

	constructor() {
		onMessengerCacheReset(() => {
			this.generation += 1;
			this.nameToURL = new Map();
			this.named.clear();
			this.beingDrawn.clear();
			this.hasLoaded = false;
			this.loading = null;
		});
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
				return;
			}
			const document: { emoji?: CustomEmojiRecord[] } = await response.json();
			if (generation !== this.generation) return;
			const drawn = document.emoji ?? [];
			this.named = new Set(drawn.map((record) => record.name));
			this.nameToURL = new Map(drawn.map((record) => [record.name, record.url]));
			this.hasLoaded = true;
		} catch {
			if (generation === this.generation) this.hasLoaded = false;
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
		for (const [name, url] of drawn) {
			if (url) filled.set(name, url);
		}
		this.nameToURL = filled;
	}

	private drawOnce(name: string): Promise<string | null> {
		const already = this.beingDrawn.get(name);
		if (already) return already;
		const drawing = fetchCustomEmojiImage(name)
			.then((image) => image?.dataURL ?? null)
			.catch(() => null);
		this.beingDrawn.set(name, drawing);
		return drawing;
	}
}

export const customEmoji = new CustomEmojiStore();
