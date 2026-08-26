import type { Locale } from './i18n/locale.svelte';

// A person's name is recorded one way and read another. The record holds it
// given name first, separated by spaces — 예시 김, John Michael Smith — because
// that is the one order every messenger, directory and mail header agrees on.
// Korean writes the family name first and joins it to the given name, so that
// is a rendering, not a second name to keep.
//
// Only a name written in Hangul is rejoined. "John Michael Smith" read by
// somebody with Korean selected is still John Michael Smith: the reader's
// language does not change how a Latin name is written.
export function personName(recorded: string, locale: Locale): string {
	const parts = recorded.trim().split(/\s+/).filter(Boolean);
	if (locale !== 'ko' || parts.length < 2 || !isHangul(recorded)) return recorded.trim();
	const family = parts[parts.length - 1];
	const given = parts.slice(0, -1);
	return family + given.join('');
}

function isHangul(name: string): boolean {
	return /^[\sᄀ-ᇿ㄰-㆏가-힯]+$/.test(name);
}
