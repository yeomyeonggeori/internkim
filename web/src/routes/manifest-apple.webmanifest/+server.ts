import manifest from '../../../static/manifest.webmanifest?raw';

export const prerender = true;

export function GET() {
	return new Response(JSON.stringify({
		...JSON.parse(manifest),
		icons: [
			{ src: '/apple-icon-512.png', sizes: '512x512', type: 'image/png' },
			{ src: '/apple-touch-icon.png', sizes: '1024x1024', type: 'image/png' }
		]
	}), { headers: { 'Content-Type': 'application/manifest+json' } });
}
