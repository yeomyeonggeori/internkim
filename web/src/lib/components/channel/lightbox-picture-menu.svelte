<script lang="ts">
	import * as ContextMenu from '$lib/components/ui/context-menu/index.js';
	import { channelText } from '$lib/i18n/channel-text';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import DownloadIcon from '@lucide/svelte/icons/download';
	import ImageIcon from '@lucide/svelte/icons/image';
	import type { Snippet } from 'svelte';
	import { toast } from 'svelte-sonner';
	import { startDownload } from './attachment-download';
	import { copyPicture } from './message-copy';

	let { source, filename, children }: { source: string; filename: string; children: Snippet } = $props();

	const text = createPageText(channelText);

	async function copy() {
		const outcome = await copyPicture(source);
		if (outcome === 'success') toast.success(text.pictureCopied);
		else toast.error(text.pictureCopyFailed);
	}
</script>

<ContextMenu.Root>
	<ContextMenu.Trigger>
		{@render children()}
	</ContextMenu.Trigger>
	<ContextMenu.Content class="w-52" portalProps={{ disabled: true }}>
		<ContextMenu.Item onSelect={copy}>
			<ImageIcon />
			<span>{text.copyPicture}</span>
		</ContextMenu.Item>
		<ContextMenu.Item onSelect={() => startDownload(source, filename)}>
			<DownloadIcon />
			<span>{text.downloadPicture}</span>
		</ContextMenu.Item>
	</ContextMenu.Content>
</ContextMenu.Root>
