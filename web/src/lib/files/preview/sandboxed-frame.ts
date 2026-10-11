import { z } from 'zod';

export const frameMessageSchema = z.discriminatedUnion('type', [
	z.object({ type: z.literal('ready') }),
	z.object({ type: z.literal('rendered'), count: z.number().int() }),
	z.object({ type: z.literal('failed'), message: z.string() })
]);

type FrameRenderer = { libraryURLs: string[]; style: string; render: string };

function frameDocument({ libraryURLs, style, render }: FrameRenderer): string {
	const scripts = libraryURLs.map((url) => `<script src="${url}"></script>`).join('\n');
	return `<!doctype html><meta charset="utf-8">
<style>html,body{margin:0;background:transparent}${style}</style>
<div id="pages"></div>
${scripts}
<script>
const pages = document.getElementById('pages');
const maxPageWidth = 960;
let bytes = null;
let renderedWidth = 0;

function availableWidth() {
	return Math.max(240, Math.min(maxPageWidth, document.documentElement.clientWidth - 32));
}

${render}

function report(error) {
	parent.postMessage({ type: 'failed', message: String(error && error.message || error) }, '*');
}

async function show() {
	renderedWidth = availableWidth();
	parent.postMessage({ type: 'rendered', count: await render(renderedWidth) }, '*');
}

addEventListener('message', (event) => {
	if (event.source !== parent || !(event.data instanceof ArrayBuffer)) return;
	bytes = event.data;
	show().catch(report);
});

let resizeTimer = 0;
addEventListener('resize', () => {
	clearTimeout(resizeTimer);
	resizeTimer = setTimeout(() => {
		if (bytes && Math.abs(availableWidth() - renderedWidth) > 24) show().catch(report);
	}, 250);
});

parent.postMessage({ type: 'ready' }, '*');
</script>`;
}

export function presentationFrameDocument(libraryURL: string): string {
	return frameDocument({
		libraryURLs: [libraryURL],
		style: '#pages{display:flex;flex-direction:column;align-items:center;padding:16px 0}',
		render: `const widescreen = 9 / 16;

async function slidesAt(width, ratio) {
	pages.replaceChildren();
	const previewer = pptxPreview.init(pages, { width, height: Math.round(width * ratio), mode: 'list' });
	await previewer.preview(bytes.slice(0));
	return previewer;
}

async function render(width) {
	let previewer = await slidesAt(width, widescreen);
	const ratio = previewer.pptx.height / previewer.pptx.width;
	if (Math.abs(ratio - widescreen) > 0.01) {
		previewer.destroy();
		previewer = await slidesAt(width, ratio);
	}
	return previewer.slideCount;
}`
	});
}

export function wordFrameDocument(zipLibraryURL: string, wordLibraryURL: string): string {
	return frameDocument({
		libraryURLs: [zipLibraryURL, wordLibraryURL],
		style: '.docx-wrapper{background:transparent!important;padding:16px 0!important}.docx-wrapper>section.docx{margin-bottom:16px!important}',
		render: `const webBullet = String.fromCharCode(0x2022);
const symbolFontBullets = [String.fromCharCode(0xf0b7), String.fromCharCode(92) + 'f0b7'];

function drawSymbolFontBulletsWithAWebGlyph() {
	for (const style of document.querySelectorAll('style')) {
		style.textContent = symbolFontBullets.reduce((text, bullet) => text.replaceAll(bullet, webBullet), style.textContent);
	}
}

async function render(width) {
	pages.replaceChildren();
	pages.style.zoom = '';
	await docx.renderAsync(bytes.slice(0), pages, undefined, { inWrapper: true, breakPages: true });
	drawSymbolFontBulletsWithAWebGlyph();
	const sections = pages.querySelectorAll('section.docx');
	const widest = Math.max(...Array.from(sections, (section) => section.offsetWidth));
	if (widest > width) pages.style.zoom = String(width / widest);
	return sections.length;
}`
	});
}
