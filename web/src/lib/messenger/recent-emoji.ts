import { catalogEmoji, quickEmojiNames } from './emoji-catalog';

const recentEmojiKey = 'internkim.messenger.recent-emoji';
const maxRecentEmojiGlyphs = 8;

function isStringArray(value: unknown): value is string[] {
	return Array.isArray(value) && value.every((item) => typeof item === 'string');
}

export function recentEmojiGlyphs(): string[] {
	try {
		const stored = window.localStorage.getItem(recentEmojiKey);
		if (!stored) return [];
		const parsed: unknown = JSON.parse(stored);
		return isStringArray(parsed) ? parsed : [];
	} catch {
		return [];
	}
}

export function rememberEmojiGlyph(glyph: string): void {
	const remainingGlyphs = recentEmojiGlyphs().filter((existing) => existing !== glyph);
	const updatedGlyphs = [glyph, ...remainingGlyphs].slice(0, maxRecentEmojiGlyphs);
	try {
		window.localStorage.setItem(recentEmojiKey, JSON.stringify(updatedGlyphs));
	} catch {
		return;
	}
}

export function quickEmojiGlyphs(count: number): string[] {
	const recentGlyphs = recentEmojiGlyphs();
	const remainingQuickGlyphs = quickEmojiNames
		.map((name) => catalogEmoji(name)?.glyph)
		.filter((glyph): glyph is string => glyph !== undefined && !recentGlyphs.includes(glyph));
	return [...recentGlyphs, ...remainingQuickGlyphs].slice(0, count);
}
