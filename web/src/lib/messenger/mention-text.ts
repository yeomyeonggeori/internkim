export const mentionLabelsContext = Symbol('mention-labels');

export type MentionPiece = {
	text: string;
	isMention: boolean;
};

export function mentionPieces(text: string, labels: string[]): MentionPiece[] {
	const wanted = [...new Set(labels.filter((label) => label.trim() !== ''))].sort(
		(left, right) => right.length - left.length
	);
	if (wanted.length === 0) return [{ text, isMention: false }];
	const pieces: MentionPiece[] = [];
	let plain = '';
	let at = 0;
	while (at < text.length) {
		const label = text[at] === '@' && wanted.find((candidate) => text.startsWith(candidate, at + 1));
		if (!label) {
			plain += text[at];
			at += 1;
			continue;
		}
		if (plain !== '') pieces.push({ text: plain, isMention: false });
		plain = '';
		pieces.push({ text: `@${label}`, isMention: true });
		at += label.length + 1;
	}
	if (plain !== '') pieces.push({ text: plain, isMention: false });
	return pieces;
}
