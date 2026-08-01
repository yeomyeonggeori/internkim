type CustomEmojiRecord = { name: string; url: string };

class CustomEmojiStore {
	nameToURL = $state<Map<string, string>>(new Map());
	private hasLoaded = false;

	async load(): Promise<void> {
		if (this.hasLoaded) return;
		this.hasLoaded = true;
		try {
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
}

export const customEmoji = new CustomEmojiStore();
