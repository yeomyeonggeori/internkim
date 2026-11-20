<script lang="ts">
	import zipLibraryURL from 'jszip/dist/jszip.min.js?url';
	import wordLibraryURL from '../../../../node_modules/docx-preview/dist/docx-preview.min.js?url';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { wordFrameDocument } from './sandboxed-frame';
	import SandboxedPreview from './sandboxed-preview.svelte';
	import { previewText } from './text';

	let {
		bytes,
		isMiniature = false,
		onAspectRatio,
		onFailure
	}: {
		bytes: ArrayBuffer;
		isMiniature?: boolean;
		onAspectRatio?: (aspectRatio: number) => void;
		onFailure: (error: unknown) => void;
	} = $props();

	const text = createPageText(previewText);
	const absolute = (url: string) => new URL(url, window.location.href).href;
	const frameDocument = wordFrameDocument(absolute(zipLibraryURL), absolute(wordLibraryURL));
</script>

<SandboxedPreview {bytes} {frameDocument} countLabel={text.pages} {isMiniature} {onAspectRatio} {onFailure} />
