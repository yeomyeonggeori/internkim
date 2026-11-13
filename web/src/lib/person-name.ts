import type { Locale } from './i18n/locale';

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
	return orderedName(recorded, locale, isHangul(recorded));
}

export function redactedPersonName(masked: string, locale: Locale): string {
	const family = masked.trim().split(/\s+/).at(-1) ?? '';
	return orderedName(masked, locale, isHangul(family));
}

function orderedName(recorded: string, locale: Locale, isKoreanName: boolean): string {
	const parts = recorded.trim().split(/\s+/).filter(Boolean);
	if (locale !== 'ko' || parts.length < 2 || !isKoreanName) return recorded.trim();
	const family = parts[parts.length - 1];
	const given = parts.slice(0, -1);
	return family + given.join('');
}

export function personNameLocale(uiLocale: Locale, companyLocale = ''): Locale {
	const companyLanguage = companyLocale.trim().toLowerCase().split('-')[0];
	return uiLocale === 'ko' || companyLanguage === 'ko' ? 'ko' : 'en';
}

function isHangul(name: string): boolean {
	return /^[\sᄀ-ᇿ㄰-㆏가-힯]+$/.test(name);
}

const PersonReachTier = {
	unreached: 0,
	exactName: 1,
	partOfName: 2,
	familyNameAndJobTitle: 3,
	jobTitle: 4
} as const;

type NamedPerson = { name: string; jobTitle: string };

export function personReachOf(hint: string, person: NamedPerson): number {
	const hintWords = wordsOf(withoutMentionSign(hint));
	const nameWords = wordsOf(person.name);
	if (hintWords.length === 0) return PersonReachTier.unreached;
	const tier = nameTier(hintWords, nameWords);
	if (tier !== PersonReachTier.unreached) return tier;
	return jobTitleTier(hintWords.join(''), nameWords, compactWords(person.jobTitle));
}

export function personReferenceNearness(hint: string, person: NamedPerson): number {
	const compactHint = compactWords(withoutMentionSign(hint));
	if (compactHint === '') return 0;
	const nameWords = wordsOf(person.name);
	const covered = Math.max(
		leadingNameFormLength(compactHint, nameWords),
		containedJobTitleLength(compactHint, nameWords, compactWords(person.jobTitle))
	);
	return covered / [...compactHint].length;
}

function nameTier(hintWords: string[], nameWords: string[]): number {
	if (nameWords.length === 0) return PersonReachTier.unreached;
	if (isExactName(hintWords, nameWords)) return PersonReachTier.exactName;
	if (isPartOfName(hintWords, nameWords)) return PersonReachTier.partOfName;
	return PersonReachTier.unreached;
}

function isExactName(hintWords: string[], nameWords: string[]): boolean {
	return (
		sortedWords(hintWords).join(' ') === sortedWords(nameWords).join(' ') ||
		hintWords.join('') === joinedName(nameWords)
	);
}

function isPartOfName(hintWords: string[], nameWords: string[]): boolean {
	if (hintWords.every((word) => nameWords.includes(word))) return true;
	const joined = joinedName(nameWords);
	return joined.includes(hintWords.join('')) || joined.includes(familyFirst(hintWords).join(''));
}

function jobTitleTier(compactHint: string, nameWords: string[], compactTitle: string): number {
	if (compactTitle === '') return PersonReachTier.unreached;
	if (compactHint === compactTitle) return PersonReachTier.jobTitle;
	const familyName = familyNameOf(nameWords);
	const isFamilyNameAndTitle =
		familyName !== '' && (compactHint === familyName + compactTitle || compactHint === compactTitle + familyName);
	return isFamilyNameAndTitle ? PersonReachTier.familyNameAndJobTitle : PersonReachTier.unreached;
}

function leadingNameFormLength(compactHint: string, nameWords: string[]): number {
	const leading = nameForms(nameWords).filter((form) => compactHint.startsWith(form));
	return Math.max(0, ...leading.map((form) => [...form].length));
}

function containedJobTitleLength(compactHint: string, nameWords: string[], compactTitle: string): number {
	if (compactTitle === '' || !compactHint.includes(compactTitle)) return 0;
	const remainder = compactHint.replace(compactTitle, '');
	const familyName = familyNameOf(nameWords);
	if (familyName !== '' && remainder.includes(familyName)) return [...(compactTitle + familyName)].length;
	return [...compactTitle].length;
}

function nameForms(nameWords: string[]): string[] {
	if (nameWords.length < 2) return [joinedName(nameWords)];
	return [joinedName(nameWords), nameWords.slice(0, -1).join('')];
}

function joinedName(nameWords: string[]): string {
	if (nameWords.length < 2) return nameWords.join('');
	return familyFirst(nameWords).join('');
}

function familyFirst(words: string[]): string[] {
	return [words[words.length - 1], ...words.slice(0, -1)];
}

function familyNameOf(nameWords: string[]): string {
	return nameWords.length < 2 ? '' : nameWords[nameWords.length - 1];
}

function withoutMentionSign(hint: string): string {
	return hint.trim().replace(/^@/, '');
}

function wordsOf(text: string): string[] {
	return text.toLowerCase().split(/\s+/).filter(Boolean);
}

function compactWords(text: string): string {
	return wordsOf(text).join('');
}

function sortedWords(words: string[]): string[] {
	return [...words].sort();
}
