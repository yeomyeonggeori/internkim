import { fetchLinkPreview, type LinkPreview } from './messenger-api';

const previewsByURL = new Map<string, Promise<LinkPreview | null>>();

export function linkPreviewOf(url: string): Promise<LinkPreview | null> {
	const known = previewsByURL.get(url);
	if (known) return known;
	const asked = fetchLinkPreview(url).catch((error: unknown) => {
		previewsByURL.delete(url);
		throw error;
	});
	previewsByURL.set(url, asked);
	return asked;
}
