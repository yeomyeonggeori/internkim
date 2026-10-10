<script lang="ts" module>
	export type PreviewSource = { url: string; fileName: string };
</script>

<script lang="ts">
	import FileQuestionIcon from '@lucide/svelte/icons/file-question';
	import * as Empty from '$lib/components/ui/empty';
	import { Spinner } from '$lib/components/ui/spinner';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { previewKindOf } from './kind';
	import { previewText } from './text';
	import PdfPreview from './pdf-preview.svelte';
	import SpreadsheetPreview from './spreadsheet-preview.svelte';

	let { source }: { source: PreviewSource } = $props();

	const text = createPageText(previewText);
	const kind = $derived(previewKindOf(source.fileName));
	let bytes = $state.raw<ArrayBuffer | null>(null);
	let failure = $state('');
	const imageURL = $derived(bytes && kind === 'image' ? URL.createObjectURL(new Blob([bytes], { type: imageTypeOf(source.fileName) })) : '');
	const plainText = $derived(bytes && kind === 'text' ? new TextDecoder().decode(bytes) : '');

	$effect(() => {
		bytes = null;
		failure = '';
		if (kind === 'none') return;
		const requested = source;
		void fetchBytes(requested.url).then(
			(fetched) => {
				if (requested === source) bytes = fetched;
			},
			(error: unknown) => {
				if (requested === source) showFailure(error);
			}
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

	function imageTypeOf(fileName: string): string {
		return fileName.toLowerCase().endsWith('.svg') ? 'image/svg+xml' : '';
	}

	function showFailure(error: unknown) {
		console.error('preview failed', error);
		failure = text.failed;
	}
</script>

{#if kind === 'none'}
	<Empty.Root class="h-full">
		<Empty.Header>
			<Empty.Media variant="icon"><FileQuestionIcon /></Empty.Media>
			<Empty.Title>{text.unsupported}</Empty.Title>
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
	<SpreadsheetPreview {bytes} fileName={source.fileName} onFailure={showFailure} />
{:else if kind === 'image'}
	<div class="flex h-full items-center justify-center overflow-auto p-4">
		<img src={imageURL} alt={source.fileName} class="max-h-full max-w-full object-contain shadow-sm" />
	</div>
{:else}
	<pre class="h-full overflow-auto p-6 font-sans text-sm leading-relaxed break-words whitespace-pre-wrap">{plainText}</pre>
{/if}
