<script lang="ts">
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu';
	import { Button } from '$lib/components/ui/button';
	import DownloadIcon from '@lucide/svelte/icons/download';
	import EllipsisIcon from '@lucide/svelte/icons/ellipsis';
	import type { CRMText } from './text';

	type Props = {
		text: CRMText;
		class?: string;
		onExport: () => void;
	};

	let { text, class: className = '', onExport }: Props = $props();
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
	</DropdownMenu.Content>
</DropdownMenu.Root>
