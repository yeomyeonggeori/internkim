import { readLinkPreview, type LinkPreview } from '../../../../host/relay/link-preview';

const previewsByWebsite = new Map<string, LinkPreview>();

export async function websitePreviewOf(website: string): Promise<LinkPreview | null> {
	const known = previewsByWebsite.get(website);
	if (known) return known;
	const found = await readLinkPreview(website);
	if (found) previewsByWebsite.set(website, found);
	return found;
}
