import { describe, expect, test } from 'bun:test';
import { decodePage, imageAddress, reachableAddress, readMetaTags, videoOf } from './link-preview';

describe('reachableAddress', () => {
	test('an ordinary web address is fetched', () => {
		expect(reachableAddress('https://example.com/article')?.hostname).toBe('example.com');
		expect(reachableAddress('http://example.com')?.hostname).toBe('example.com');
	});

	test('only the web is fetched', () => {
		expect(reachableAddress('ftp://example.com/file')).toBeNull();
		expect(reachableAddress('file:///etc/passwd')).toBeNull();
		expect(reachableAddress('not a url')).toBeNull();
	});

	test('the company network is not fetched on a stranger behalf', () => {
		for (const link of [
			'http://localhost:8080/admin',
			'http://127.0.0.1/admin',
			'http://10.0.0.5/',
			'http://192.168.1.1/',
			'http://172.20.3.4/',
			'http://169.254.169.254/latest/meta-data',
			'http://postgres.internal/',
			'http://[::1]:8080/'
		]) {
			expect(reachableAddress(link)).toBeNull();
		}
	});
});

describe('the picture a preview names', () => {
	const page = new URL('https://example.com/articles/one');

	test('is where the page keeps it, because that is already a copy', () => {
		expect(imageAddress('https://cdn.example.com/one.png', page)).toBe('https://cdn.example.com/one.png');
	});

	test('is resolved against the page when the page names it relatively', () => {
		expect(imageAddress('../covers/one.png', page)).toBe('https://example.com/covers/one.png');
	});

	test('is left out when the page names none', () => {
		expect(imageAddress(undefined, page)).toBe('');
		expect(imageAddress('', page)).toBe('');
	});

	// A reader's browser opens this address, so a page naming somewhere on their
	// own network would have them reach it.
	test('is left out when it points at a network the reader should not be aimed at', () => {
		expect(imageAddress('http://169.254.169.254/latest/meta-data', page)).toBe('');
		expect(imageAddress('http://192.168.1.1/', page)).toBe('');
		expect(imageAddress('file:///etc/passwd', page)).toBe('');
	});
});

describe('decodePage', () => {
	const euckrHangul = new Uint8Array([0xc7, 0xd1, 0xb1, 0xb9, 0xbe, 0xee]);

	test('reads the charset the page declares in its header', () => {
		expect(decodePage(euckrHangul, 'text/html; charset=euc-kr')).toBe('한국어');
	});

	test('falls back to the charset in the document when the header is silent', () => {
		const document = new TextEncoder().encode('<meta charset="utf-8"><title>한국어</title>');
		expect(decodePage(document, 'text/html')).toContain('한국어');
	});

	test('an unknown charset does not throw', () => {
		expect(decodePage(new TextEncoder().encode('hi'), 'text/html; charset=nonsense-9')).toBe('hi');
	});
});

describe('readMetaTags', () => {
	test('reads a value the page wrote as character references as the text it stands for', async () => {
		const tags = await readMetaTags(
			'<meta property="og:title" content="&#xc0d8;&#xd50c; on Instagram: &quot;hi&quot;">' +
				'<meta property="og:image" content="https://cdn.example.com/one.jpg?a=1&amp;b=2">'
		);
		expect(tags.get('og:title')).toBe('샘플 on Instagram: "hi"');
		expect(tags.get('og:image')).toBe('https://cdn.example.com/one.jpg?a=1&b=2');
	});

	test('reads the document title the same way', async () => {
		const tags = await readMetaTags('<title>Tom &amp; Jerry &#8212; one</title>');
		expect(tags.get('title')).toBe('Tom & Jerry — one');
	});
});

describe('the video a preview names', () => {
	const page = new URL('https://example.com/watch/one');
	const tagsOf = (entries: Record<string, string>) => new Map(Object.entries(entries));

	test('is a file the reader plays when the page declares a video type', () => {
		const tags = tagsOf({
			'og:video': 'http://cdn.example.com/one.mp4',
			'og:video:secure_url': 'https://cdn.example.com/one.mp4',
			'og:video:type': 'video/mp4',
			'og:video:width': '1280',
			'og:video:height': '720'
		});
		expect(videoOf(tags, page)).toEqual({ url: 'https://cdn.example.com/one.mp4', kind: 'file', width: 1280, height: 720 });
	});

	test('is a player the site serves when the page declares an HTML video', () => {
		const tags = tagsOf({ 'og:video:url': 'https://player.example.com/embed/one', 'og:video:type': 'text/html' });
		expect(videoOf(tags, page)).toEqual({ url: 'https://player.example.com/embed/one', kind: 'player', width: 0, height: 0 });
	});

	test('falls back to the twitter player when the open graph video is in a form nobody plays', () => {
		const tags = tagsOf({
			'og:video': 'https://example.com/one.swf',
			'og:video:type': 'application/x-shockwave-flash',
			'twitter:player': 'https://player.example.com/embed/one',
			'twitter:player:width': '640',
			'twitter:player:height': '360'
		});
		expect(videoOf(tags, page)).toEqual({ url: 'https://player.example.com/embed/one', kind: 'player', width: 640, height: 360 });
	});

	test('is left out when the page names none or names one without a type', () => {
		expect(videoOf(tagsOf({}), page)).toBeNull();
		expect(videoOf(tagsOf({ 'og:video': 'https://cdn.example.com/one.mp4' }), page)).toBeNull();
	});

	test('is left out when it is not served over https', () => {
		const tags = tagsOf({ 'og:video': 'http://cdn.example.com/one.mp4', 'og:video:type': 'video/mp4' });
		expect(videoOf(tags, page)).toBeNull();
	});

	test('is left out when it points at a network the reader should not be aimed at', () => {
		const tags = tagsOf({ 'twitter:player': 'https://192.168.1.1/player' });
		expect(videoOf(tags, page)).toBeNull();
	});
});
