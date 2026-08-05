import { isSupabaseConfigured } from '$lib/supabase';
import { fetchCustomEmoji } from '$lib/messenger/messenger-api';

type CustomEmojiRecord = { name: string; url: string };

class CustomEmojiStore {
	nameToURL = $state<Map<string, string>>(new Map());
	private hasLoaded = false;

	async load(): Promise<void> {
		if (this.hasLoaded) return;
		this.hasLoaded = true;
		try {
			if (isSupabaseConfigured()) {
				this.nameToURL = await this.fromCompanyApp();
				return;
			}
			const response = await fetch('/agent/api/custom-emoji', { credentials: 'include' });
			if (!response.ok) {
				this.hasLoaded = false;
				return;
			}
			const document: { emoji?: CustomEmojiRecord[] } = await response.json();
			this.nameToURL = new Map((document.emoji ?? []).map((record) => [record.name, record.url]));
		} catch {
			this.hasLoaded = false;
		}
	}

	private async fromCompanyApp(): Promise<Map<string, string>> {
		for (let attempt = 0; attempt < 4; attempt += 1) {
			try {
				const drawn = await fetchCustomEmoji();
				if (drawn.length > 0) return new Map(drawn.map((record) => [record.name, record.url]));
			} catch {
				// the app answers once its presence reaches this page
			}
			await new Promise((wait) => setTimeout(wait, 2000));
		}
		throw new Error('the app did not hand over its emoji');
	}
}

export const customEmoji = new CustomEmojiStore();
