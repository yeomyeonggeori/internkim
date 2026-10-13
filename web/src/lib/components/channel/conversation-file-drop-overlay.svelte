<script lang="ts">
	import * as Empty from '$lib/components/ui/empty/index.js';
	import PaperclipIcon from '@lucide/svelte/icons/paperclip';

	let { visible, title, description }: { visible: boolean; title: string; description: string } = $props();
</script>

{#if visible}
	<div class="bg-background/80 pointer-events-none absolute inset-0 z-30 flex p-3 backdrop-blur-sm">
		<Empty.Root class="border-primary/50 border-2">
			<Empty.Header>
				<Empty.Media variant="icon"><PaperclipIcon /></Empty.Media>
				<Empty.Title>{title}</Empty.Title>
				<Empty.Description>{description}</Empty.Description>
			</Empty.Header>
		</Empty.Root>
	</div>
{/if}
