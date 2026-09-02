export function typoNearness(hint: string, value: string): number {
	const asked = [...hint];
	const held = [...value];
	if (asked.length === 0 || held.length === 0) return 0;
	const longest = Math.max(asked.length, held.length);
	const distance = editDistance(asked, held);
	if (distance > allowedTypoDistance(longest)) return 0;
	return 1 - distance / longest;
}

export function titleNearness(hint: string, value: string): number {
	const asked = characterBigrams(hint);
	const held = characterBigrams(value);
	if (asked.length === 0 || held.length === 0) return 0;

	const remaining = new Map<string, number>();
	for (const bigram of held) remaining.set(bigram, (remaining.get(bigram) ?? 0) + 1);

	let shared = 0;
	for (const bigram of asked) {
		const left = remaining.get(bigram) ?? 0;
		if (left === 0) continue;
		remaining.set(bigram, left - 1);
		shared += 1;
	}
	return (2 * shared) / (asked.length + held.length);
}

export function emailNearness(hint: string, email: string): number {
	const asked = addressParts(hint);
	const held = addressParts(email);
	if (!asked || !held) return 0;
	if (asked.local === held.local) {
		return asked.domain === held.domain ? 1 : typoNearness(asked.domain, held.domain);
	}
	if (asked.domain !== held.domain) return 0;
	return typoNearness(asked.local, held.local);
}

function allowedTypoDistance(length: number): number {
	return length <= 4 ? 1 : 2;
}

function addressParts(value: string): { local: string; domain: string } | null {
	const at = value.indexOf('@');
	if (at <= 0 || at === value.length - 1) return null;
	return { local: value.slice(0, at), domain: value.slice(at + 1) };
}

function editDistance(first: string[], second: string[]): number {
	let previousRow = second.map((_, column) => column).concat(second.length);
	for (let row = 1; row <= first.length; row += 1) {
		const currentRow = [row];
		for (let column = 1; column <= second.length; column += 1) {
			const substitution = first[row - 1] === second[column - 1] ? 0 : 1;
			currentRow.push(
				Math.min(
					previousRow[column] + 1,
					currentRow[column - 1] + 1,
					previousRow[column - 1] + substitution
				)
			);
		}
		previousRow = currentRow;
	}
	return previousRow[second.length];
}

function characterBigrams(value: string): string[] {
	const characters = [...value.split(/\s+/).join('')];
	const bigrams: string[] = [];
	for (let index = 0; index + 1 < characters.length; index += 1) {
		bigrams.push(characters[index] + characters[index + 1]);
	}
	return bigrams;
}
