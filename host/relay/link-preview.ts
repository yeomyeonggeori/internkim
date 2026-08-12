export type LinkPreviewImage = { bytes: Uint8Array; contentType: string };

export type LinkPreview = {
	url: string;
	title: string;
	description: string;
	siteName: string;
	image: LinkPreviewImage | null;
};

const pageByteCap = 512 * 1024;
const imageByteCap = 256 * 1024;
const timeoutMillisecond = 5000;

export async function readLinkPreview(link: string): Promise<LinkPreview | null> {
	const address = reachableAddress(link);
	if (!address) return null;

	const page = await readCapped(address, pageByteCap, 'text/html');
	if (!page) return null;

	const tags = await readMetaTags(decodePage(page.bytes, page.declaredType));
	const title = tags.get('og:title') ?? tags.get('title') ?? '';
	if (!title) return null;

	return {
		url: address.toString(),
		title,
		description: tags.get('og:description') ?? tags.get('description') ?? '',
		siteName: tags.get('og:site_name') ?? address.hostname,
		image: await readImage(tags.get('og:image'), address)
	};
}

export function reachableAddress(link: string): URL | null {
	const address = URL.parse(link);
	if (!address) return null;
	if (address.protocol !== 'http:' && address.protocol !== 'https:') return null;
	if (isLocalHostname(address.hostname)) return null;
	return address;
}

function isLocalHostname(hostname: string): boolean {
	const name = hostname.toLowerCase().replace(/^\[|\]$/g, '');
	if (name === 'localhost' || name.endsWith('.localhost') || name.endsWith('.local') || name.endsWith('.internal')) {
		return true;
	}
	if (name === '::1' || name.startsWith('fe80:') || name.startsWith('fc') || name.startsWith('fd')) return true;
	const octets = name.split('.').map(Number);
	if (octets.length !== 4 || octets.some((octet) => !Number.isInteger(octet) || octet < 0 || octet > 255)) return false;
	const [first, second] = octets;
	if (first === 127 || first === 10 || first === 0) return true;
	if (first === 192 && second === 168) return true;
	if (first === 172 && second >= 16 && second <= 31) return true;
	if (first === 169 && second === 254) return true;
	return false;
}

async function readCapped(
	address: URL,
	byteCap: number,
	wantedType: string
): Promise<{ bytes: Uint8Array; contentType: string; declaredType: string } | null> {
	const response = await fetch(address, {
		signal: AbortSignal.timeout(timeoutMillisecond),
		redirect: 'follow',
		headers: { accept: `${wantedType},*/*;q=0.5`, 'user-agent': 'internkim-link-preview' }
	}).catch(() => null);
	if (!response?.ok || !response.body) return null;

	const declaredType = response.headers.get('content-type') ?? '';
	const contentType = declaredType.split(';')[0].trim();
	if (!contentType.startsWith(wantedType)) {
		await response.body.cancel().catch(() => undefined);
		return null;
	}

	const chunks: Uint8Array[] = [];
	let read = 0;
	for await (const chunk of response.body) {
		chunks.push(chunk);
		read += chunk.byteLength;
		if (read >= byteCap) break;
	}
	await response.body.cancel().catch(() => undefined);
	return { bytes: Bun.concatArrayBuffers(chunks, byteCap, true), contentType, declaredType };
}

export function decodePage(bytes: Uint8Array, declaredType: string): string {
	const fromHeader = charsetIn(declaredType);
	if (fromHeader) return decodeWith(bytes, fromHeader);

	const guessed = decodeWith(bytes, 'utf-8');
	const fromDocument = charsetIn(guessed.slice(0, 4096));
	return fromDocument ? decodeWith(bytes, fromDocument) : guessed;
}

function charsetIn(text: string): string {
	return /charset\s*=\s*["']?([\w-]+)/i.exec(text)?.[1] ?? '';
}

function decodeWith(bytes: Uint8Array, label: string): string {
	try {
		return new TextDecoder(label).decode(bytes);
	} catch {
		return new TextDecoder().decode(bytes);
	}
}

async function readMetaTags(html: string): Promise<Map<string, string>> {
	const tags = new Map<string, string>();
	let title = '';
	await new HTMLRewriter()
		.on('meta', {
			element(tag) {
				const key = tag.getAttribute('property') ?? tag.getAttribute('name');
				const value = tag.getAttribute('content');
				if (key && value && !tags.has(key)) tags.set(key, value);
			}
		})
		.on('title', {
			text(chunk) {
				title += chunk.text;
			}
		})
		.transform(new Response(html))
		.text();
	if (title.trim() && !tags.has('title')) tags.set('title', title.trim());
	return tags;
}

async function readImage(link: string | undefined, page: URL): Promise<LinkPreviewImage | null> {
	if (!link) return null;
	const address = reachableAddress(new URL(link, page).toString());
	if (!address) return null;
	const picture = await readCapped(address, imageByteCap, 'image/');
	if (!picture) return null;
	return { bytes: picture.bytes, contentType: picture.contentType };
}
