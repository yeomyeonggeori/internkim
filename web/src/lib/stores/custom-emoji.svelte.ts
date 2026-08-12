import { isSupabaseConfigured } from '$lib/supabase';
import { fetchCustomEmojiImage, fetchCustomEmojiNames } from '$lib/messenger/messenger-api';

type CustomEmojiRecord = { name: string; url: string };

class CustomEmojiStore {
	nameToURL = $state<Map<string, string>>(new Map());
	private named = new Set<string>();
	private beingDrawn = new Map<string, Promise<string | null>>();
	private hasLoaded = false;

	async load(): Promise<void> {
		if (this.hasLoaded) return;
		this.hasLoaded = true;
		try {
			if (isSupabaseConfigured()) {
				this.named = new Set(await this.namesFromCompanyApp());
				return;
			}
			const response = await fetch('/agent/api/custom-emoji', { credentials: 'include' });
			if (!response.ok) {
				this.hasLoaded = false;
				return;
			}
			const document: { emoji?: CustomEmojiRecord[] } = await response.json();
			const drawn = document.emoji ?? [];
			this.named = new Set(drawn.map((record) => record.name));
			this.nameToURL = new Map(drawn.map((record) => [record.name, record.url]));
		} catch {
			this.hasLoaded = false;
		}
	}

	async draw(names: Iterable<string>): Promise<void> {
		const wanted = [...new Set(names)].filter(
			(name) => this.named.has(name) && !this.nameToURL.has(name)
		);
		if (wanted.length === 0) return;
		const drawn = await Promise.all(
			wanted.map(async (name) => [name, await this.drawOnce(name)] as const)
		);
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

	private async namesFromCompanyApp(): Promise<string[]> {
		for (let attempt = 0; attempt < 4; attempt += 1) {
			try {
				const named = await fetchCustomEmojiNames();
				if (named.length > 0) return named;
			} catch {
			}
			await new Promise((wait) => setTimeout(wait, 2000));
		}
		throw new Error('the app did not hand over its emoji');
	}
}

export const customEmoji = new CustomEmojiStore();
