export const replyLanguageTags = ['ko', 'en', 'ja', 'zh', 'es', 'fr', 'de', 'pt', 'ru', 'vi', 'id', 'th'];

export function languageAutonym(tag: string): string {
	try {
		const autonym = new Intl.DisplayNames([tag], { type: 'language' }).of(tag);
		return autonym && autonym !== tag ? autonym : tag;
	} catch {
		return tag;
	}
}

export function replyLanguageOptions(selected: string): { value: string; label: string }[] {
	const tags = selected && !replyLanguageTags.includes(selected) ? [selected, ...replyLanguageTags] : replyLanguageTags;
	return tags.map((tag) => ({ value: tag, label: languageAutonym(tag) }));
}
