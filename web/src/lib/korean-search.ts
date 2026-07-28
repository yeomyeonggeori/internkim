const koreanInitials = ['ㄱ', 'ㄲ', 'ㄴ', 'ㄷ', 'ㄸ', 'ㄹ', 'ㅁ', 'ㅂ', 'ㅃ', 'ㅅ', 'ㅆ', 'ㅇ', 'ㅈ', 'ㅉ', 'ㅊ', 'ㅋ', 'ㅌ', 'ㅍ', 'ㅎ'];
const koreanMedials = ['ㅏ', 'ㅐ', 'ㅑ', 'ㅒ', 'ㅓ', 'ㅔ', 'ㅕ', 'ㅖ', 'ㅗ', 'ㅘ', 'ㅙ', 'ㅚ', 'ㅛ', 'ㅜ', 'ㅝ', 'ㅞ', 'ㅟ', 'ㅠ', 'ㅡ', 'ㅢ', 'ㅣ'];
const koreanFinals = ['', 'ㄱ', 'ㄲ', 'ㄳ', 'ㄴ', 'ㄵ', 'ㄶ', 'ㄷ', 'ㄹ', 'ㄺ', 'ㄻ', 'ㄼ', 'ㄽ', 'ㄾ', 'ㄿ', 'ㅀ', 'ㅁ', 'ㅂ', 'ㅄ', 'ㅅ', 'ㅆ', 'ㅇ', 'ㅈ', 'ㅊ', 'ㅋ', 'ㅌ', 'ㅍ', 'ㅎ'];
const firstSyllableCode = 0xac00;
const lastSyllableCode = 0xd7a3;
const medialCount = koreanMedials.length;
const finalCount = koreanFinals.length;

function decomposeCharacter(character: string) {
	const code = character.charCodeAt(0);
	if (code < firstSyllableCode || code > lastSyllableCode) return character;
	const syllableIndex = code - firstSyllableCode;
	const initial = koreanInitials[Math.floor(syllableIndex / (medialCount * finalCount))];
	const medial = koreanMedials[Math.floor(syllableIndex / finalCount) % medialCount];
	const final = koreanFinals[syllableIndex % finalCount];
	return `${initial}${medial}${final}`;
}

function decomposeText(text: string) {
	let jamo = '';
	const characterOffsets: number[] = [];
	for (const character of text) {
		characterOffsets.push(jamo.length);
		jamo += decomposeCharacter(character);
	}
	return { jamo, characterOffsets };
}

function queryJamoVariants(query: string) {
	const jamo = decomposeText(query).jamo;
	const lastCharacter = [...query].at(-1) ?? '';
	const lastFinal = koreanFinals[(lastCharacter.charCodeAt(0) - firstSyllableCode) % finalCount];
	if (!decomposeCharacter(lastCharacter).endsWith(lastFinal) || !lastFinal) return [jamo];
	return [jamo, jamo.slice(0, -lastFinal.length)];
}

export function matchesKoreanSearch(text: string, query: string) {
	const normalizedQuery = query.trim().toLowerCase();
	if (!normalizedQuery) return true;
	const normalizedText = text.toLowerCase();
	if (normalizedText.includes(normalizedQuery)) return true;
	const decomposedText = decomposeText(normalizedText);
	return queryJamoVariants(normalizedQuery).some((queryJamo) =>
		decomposedText.characterOffsets.some((offset) => decomposedText.jamo.startsWith(queryJamo, offset))
	);
}

export function koreanSearchScore(value: string, query: string, keywords: string[] = []) {
	const haystacks = [value, ...keywords];
	return haystacks.some((haystack) => matchesKoreanSearch(haystack, query)) ? 1 : 0;
}
