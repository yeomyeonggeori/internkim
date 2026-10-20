<script lang="ts">
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { FilesState, setFilesState } from './files-context.svelte';
	import { filesText } from './text';

	let { children } = $props();

	const text = createPageText(filesText);
	const files = new FilesState(text.loadFailed, text.companyComputerOffline);
	setFilesState(files);
</script>

<section aria-label={text.title} class="flex min-h-0 flex-1 flex-col overflow-hidden">
	{@render children()}
</section>
