export const mentionLabelsContext = Symbol('mention-labels');

export type MentionLabel = {
	label: string;
	externalID?: string;
};

export type MentionPiece = {
	text: string;
	isMention: boolean;
	externalID?: string;
};

export function mentionPieces(text: string, mentions: MentionLabel[]): MentionPiece[] {
	const wanted = [
		...new Map(mentions.filter((mention) => mention.label.trim() !== '').map((mention) => [mention.label, mention])).values()
	].sort((left, right) => right.label.length - left.label.length);
	if (wanted.length === 0) return [{ text, isMention: false }];
	const pieces: MentionPiece[] = [];
	let plain = '';
	let at = 0;
	while (at < text.length) {
		const mention = text[at] === '@' ? wanted.find((candidate) => text.startsWith(candidate.label, at + 1)) : undefined;
		if (!mention) {
			plain += text[at];
			at += 1;
			continue;
		}
		if (plain !== '') pieces.push({ text: plain, isMention: false });
		plain = '';
		pieces.push({
			text: `@${mention.label}`,
			isMention: true,
			...(mention.externalID ? { externalID: mention.externalID } : {})
		});
		at += mention.label.length + 1;
	}
	if (plain !== '') pieces.push({ text: plain, isMention: false });
	return pieces;
}
