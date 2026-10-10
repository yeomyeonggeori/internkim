import { z } from 'zod';

export const presentationFrameMessageSchema = z.discriminatedUnion('type', [
	z.object({ type: z.literal('ready') }),
	z.object({ type: z.literal('rendered'), slideCount: z.number().int() }),
	z.object({ type: z.literal('failed'), message: z.string() })
]);

export function presentationFrameDocument(libraryURL: string): string {
	return `<!doctype html><meta charset="utf-8">
<style>html,body{margin:0;background:transparent}#slides{display:flex;flex-direction:column;align-items:center;padding:16px 0}</style>
<div id="slides"></div>
<script src="${libraryURL}"></script>
<script>
const slides = document.getElementById('slides');
const widescreen = 9 / 16;
const maxSlideWidth = 960;
let bytes = null;
let renderedWidth = 0;

async function renderAt(width, ratio) {
	slides.replaceChildren();
	const previewer = pptxPreview.init(slides, { width, height: Math.round(width * ratio), mode: 'list' });
	await previewer.preview(bytes.slice(0));
	return previewer;
}

async function render() {
	const width = Math.max(240, Math.min(maxSlideWidth, document.documentElement.clientWidth - 32));
	renderedWidth = width;
	let previewer = await renderAt(width, widescreen);
	const ratio = previewer.pptx.height / previewer.pptx.width;
	if (Math.abs(ratio - widescreen) > 0.01) {
		previewer.destroy();
		previewer = await renderAt(width, ratio);
	}
	parent.postMessage({ type: 'rendered', slideCount: previewer.slideCount }, '*');
}

function report(error) {
	parent.postMessage({ type: 'failed', message: String(error && error.message || error) }, '*');
}

addEventListener('message', (event) => {
	if (event.source !== parent || !(event.data instanceof ArrayBuffer)) return;
	bytes = event.data;
	render().catch(report);
});

let resizeTimer = 0;
addEventListener('resize', () => {
	clearTimeout(resizeTimer);
	resizeTimer = setTimeout(() => {
		if (bytes && Math.abs(Math.min(maxSlideWidth, document.documentElement.clientWidth - 32) - renderedWidth) > 24) render().catch(report);
	}, 250);
});

parent.postMessage({ type: 'ready' }, '*');
</script>`;
}
