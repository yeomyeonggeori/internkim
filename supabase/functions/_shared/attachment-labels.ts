import type { Locale } from './member-locale.ts';

const labelsByLocale: Record<Locale, { image: string; video: string; file: string }> = {
	ko: { image: '[사진]', video: '[동영상]', file: '[파일]' },
	en: { image: '[Photo]', video: '[Video]', file: '[File]' }
};

export function bodyWithAttachmentLabels(body: string, attachmentKinds: string[], locale: Locale): string {
	const labels = labelsByLocale[locale];
	const named = [...new Set(attachmentKinds)].map((kind) =>
		kind === 'image' || kind === 'video' ? labels[kind] : labels.file
	);
	return [body.trim(), ...new Set(named)].filter((part) => part !== '').join(' ');
}
