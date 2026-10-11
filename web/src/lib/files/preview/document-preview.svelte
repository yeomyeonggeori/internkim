<script lang="ts">
	import { onMount } from 'svelte';
	import SvelteMarkdown from '@humanspeak/svelte-markdown';
	import FileQuestionIcon from '@lucide/svelte/icons/file-question';
	import * as Code from '$lib/components/ui/code';
	import * as Empty from '$lib/components/ui/empty';
	import { Spinner } from '$lib/components/ui/spinner';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { codeLanguageForFile } from '$lib/files/view';
	import { isTooLargeToPreview, previewKindOf } from './kind';
	import { previewText } from './text';
	import PdfPreview from './pdf-preview.svelte';
	import SpreadsheetPreview from './spreadsheet-preview.svelte';
	import PresentationPreview from './presentation-preview.svelte';
	import WordPreview from './word-preview.svelte';
	import HtmlPreview from './html-preview.svelte';

	let {
		fileName,
		size,
		readURL
	}: { fileName: string; size?: number; readURL: () => Promise<string> } = $props();

	const text = createPageText(previewText);
	const kind = $derived(previewKindOf(fileName));
	const isTooLarge = $derived(size !== undefined && isTooLargeToPreview(fileName, size));
	const textualKinds = ['markdown', 'code', 'text'];
	let bytes = $state.raw<ArrayBuffer | null>(null);
	let failure = $state('');
	const imageURL = $derived(bytes && kind === 'image' ? URL.createObjectURL(new Blob([bytes], { type: imageTypeOf(fileName) })) : '');
	const plainText = $derived(bytes && textualKinds.includes(kind) ? new TextDecoder().decode(bytes) : '');

	onMount(() => {
		if (kind === 'none' || isTooLarge) return;
		readURL().then(fetchBytes).then(
			(fetched) => (bytes = fetched),
			showFailure
		);
	});

	$effect(() => {
		const url = imageURL;
		return () => {
			if (url) URL.revokeObjectURL(url);
		};
	});

	async function fetchBytes(url: string): Promise<ArrayBuffer> {
		const response = await fetch(url, { cache: 'no-store' });
		if (!response.ok) throw new Error(`the file answered ${response.status}`);
		return response.arrayBuffer();
	}

	function imageTypeOf(name: string): string {
		return name.toLowerCase().endsWith('.svg') ? 'image/svg+xml' : '';
	}

	function showFailure(error: unknown) {
		console.error('preview failed', error);
		failure = text.failed;
	}
</script>

{#if kind === 'none' || isTooLarge}
	<Empty.Root class="h-full">
		<Empty.Header>
			<Empty.Media variant="icon"><FileQuestionIcon /></Empty.Media>
			<Empty.Title>{isTooLarge ? text.tooLarge : text.unsupported}</Empty.Title>
			<Empty.Description>{text.unsupportedHint}</Empty.Description>
		</Empty.Header>
	</Empty.Root>
{:else if failure}
	<p role="alert" class="text-destructive flex h-full items-center justify-center p-6 text-sm">{failure}</p>
{:else if !bytes}
	<div role="status" aria-label={text.loading} class="flex h-full items-center justify-center"><Spinner /></div>
{:else if kind === 'pdf'}
	<PdfPreview {bytes} onFailure={showFailure} />
{:else if kind === 'spreadsheet'}
	<SpreadsheetPreview {bytes} fileName={fileName} onFailure={showFailure} />
{:else if kind === 'presentation'}
	<PresentationPreview {bytes} onFailure={showFailure} />
{:else if kind === 'word'}
	<WordPreview {bytes} onFailure={showFailure} />
{:else if kind === 'html'}
	<HtmlPreview {bytes} />
{:else if kind === 'image'}
	<div class="flex h-full items-center justify-center overflow-auto p-4">
		<img src={imageURL} alt={fileName} class="max-h-full max-w-full object-contain shadow-sm" />
	</div>
{:else if kind === 'markdown'}
	<div class="h-full overflow-auto p-6">
		<article class="prose prose-sm dark:prose-invert mx-auto max-w-3xl break-words"><SvelteMarkdown source={plainText} /></article>
	</div>
{:else if kind === 'code'}
	<div class="h-full overflow-auto p-4">
		<Code.Root code={plainText} lang={codeLanguageForFile(fileName) ?? 'bash'} class="text-xs" />
	</div>
{:else}
	<pre class="h-full overflow-auto p-6 font-sans text-sm leading-relaxed break-words whitespace-pre-wrap">{plainText}</pre>
{/if}
