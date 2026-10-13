<script lang="ts">
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu';
	import { Button } from '$lib/components/ui/button';
	import DownloadIcon from '@lucide/svelte/icons/download';
	import UploadIcon from '@lucide/svelte/icons/upload';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { crmImportText } from './crm-import-text';
	import EllipsisIcon from '@lucide/svelte/icons/ellipsis';
	import type { CRMText } from './text';

	type Props = {
		text: CRMText;
		class?: string;
		onExport: () => void;
		onImport?: () => void;
	};

	let { text, class: className = '', onExport, onImport }: Props = $props();
	const importText = createPageText(crmImportText);
</script>

<DropdownMenu.Root>
	<DropdownMenu.Trigger>
		{#snippet child({ props })}
			<Button {...props} type="button" variant="outline" size="icon" class={className} aria-label={text.more}>
				<EllipsisIcon />
			</Button>
		{/snippet}
	</DropdownMenu.Trigger>
	<DropdownMenu.Content align="end">
		<DropdownMenu.Item onSelect={onExport}>
			<DownloadIcon />
			{text.export.menuItem}
		</DropdownMenu.Item>
		{#if onImport}
			<DropdownMenu.Item onSelect={onImport}>
				<UploadIcon />
				{importText.menuItem}
			</DropdownMenu.Item>
		{/if}
	</DropdownMenu.Content>
</DropdownMenu.Root>
