<script lang="ts">
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import * as UnderlineTabs from '$lib/components/ui/underline-tabs';
	import { companyPathOf, companySlugOf, routePathOf } from '$lib/company-path';
	import { FilesState, setFilesState } from './files-context.svelte';
	import { filesText } from './text';

	let { children } = $props();

	const text = createPageText(filesText);
	const files = new FilesState(text.loadFailed);
	setFilesState(files);

	const activeTab = $derived(
		routePathOf(page.url.pathname).startsWith('/files/data-room') ? 'data-room' : 'workspace'
	);
	function openTab(value: string) {
		const path = value === 'data-room' ? '/files/data-room/' : '/files/';
		void goto(companyPathOf(companySlugOf(page.url.pathname), path));
	}
</script>

<UnderlineTabs.Root value={activeTab} onValueChange={openTab} class="min-h-0 flex-1 gap-0">
	<div class="shrink-0 border-b px-4 md:px-6">
		<UnderlineTabs.List aria-label={text.title}>
			<UnderlineTabs.Trigger value="workspace">{text.workspace}</UnderlineTabs.Trigger>
			<UnderlineTabs.Trigger value="data-room">{text.dataRoom}</UnderlineTabs.Trigger>
		</UnderlineTabs.List>
	</div>
	<UnderlineTabs.Content value={activeTab} class="min-h-0 flex-1 overflow-hidden">
		{@render children()}
	</UnderlineTabs.Content>
</UnderlineTabs.Root>
