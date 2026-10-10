<script lang="ts">
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { previewText } from './text';

	let { bytes }: { bytes: ArrayBuffer } = $props();

	const text = createPageText(previewText);
	const html = $derived(new TextDecoder().decode(bytes));
</script>

<div class="flex h-full min-h-0 flex-col">
	<p class="text-muted-foreground bg-background/80 border-b px-3 py-1.5 text-xs">{text.htmlIsolated}</p>
	<iframe title={text.htmlPage} sandbox="" srcdoc={html} class="min-h-0 w-full flex-1 border-0 bg-white"></iframe>
</div>
