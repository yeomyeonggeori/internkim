<script lang="ts">
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { onMount } from 'svelte';
	import { FilesState, setFilesState } from './files-context.svelte';
	import FilesSidebar from './files-sidebar.svelte';
	import { filesText } from './text';

	let { children } = $props();

	const text = createPageText(filesText);
	const files = new FilesState(text.loadFailed);
	setFilesState(files);

	onMount(() => files.loadRoots());
</script>

<div class="flex min-h-0 flex-1">
	<FilesSidebar />

	<div class="min-w-0 flex-1 overflow-y-auto p-6">
		{@render children()}
	</div>
</div>
