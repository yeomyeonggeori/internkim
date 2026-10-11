<script lang="ts">
	import libraryURL from 'pptx-preview/dist/pptx-preview.umd.js?url';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { presentationFrameDocument } from './sandboxed-frame';
	import SandboxedPreview from './sandboxed-preview.svelte';
	import { previewText } from './text';

	let { bytes, onFailure }: { bytes: ArrayBuffer; onFailure: (error: unknown) => void } = $props();

	const text = createPageText(previewText);
	const frameDocument = presentationFrameDocument(new URL(libraryURL, window.location.href).href);
</script>

<SandboxedPreview {bytes} {frameDocument} countLabel={text.slides} {onFailure} />
