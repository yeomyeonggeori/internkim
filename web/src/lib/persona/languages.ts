export const replyLanguageTags = ['ko', 'en', 'ja', 'zh', 'es', 'fr', 'de', 'pt', 'ru', 'vi', 'id', 'th'];

export function languageAutonym(tag: string): string {
	try {
		const autonym = new Intl.DisplayNames([tag], { type: 'language' }).of(tag);
		return autonym && autonym !== tag ? autonym : tag;
	} catch {
		return tag;
	}
}

export function defaultCallMe(name: string, languageTag: string): string {
	const parts = name.trim().split(/\s+/).filter(Boolean);
	const firstName = parts.slice(0, Math.max(1, parts.length - 1)).join(' ');
	if (!firstName) return '';
	const baseLanguage = languageTag.trim().toLowerCase().split('-')[0] ?? '';
	return baseLanguage === 'ko' ? `${firstName} 님` : firstName;
}

export function replyLanguageOptions(selected: string): { value: string; label: string }[] {
	const tags = selected && !replyLanguageTags.includes(selected) ? [selected, ...replyLanguageTags] : replyLanguageTags;
	return tags.map((tag) => ({ value: tag, label: languageAutonym(tag) }));
}
