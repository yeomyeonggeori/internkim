import { get as glyphOfName } from 'node-emoji';

// A messenger names an emoji; a screen shows the character. node-emoji knows the
// plain names, but not the skin-toned ones a messenger writes as a suffix —
// +1_light_skin_tone is 👍 with U+1F3FB after it. Without this the reaction
// showed as the name itself, spelled out with underscores.
const skinTones: Record<string, string> = {
	light: '\u{1F3FB}',
	medium_light: '\u{1F3FC}',
	medium: '\u{1F3FD}',
	medium_dark: '\u{1F3FE}',
	dark: '\u{1F3FF}'
};

export function glyphOfEmojiName(name: string): string | undefined {
	const plain = glyphOfName(name);
	if (plain) return plain;

	for (const [tone, modifier] of Object.entries(skinTones)) {
		const suffix = `_${tone}_skin_tone`;
		if (!name.endsWith(suffix)) continue;
		const base = glyphOfName(name.slice(0, -suffix.length));
		if (base) return base + modifier;
	}
	return undefined;
}
