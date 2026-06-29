<script lang="ts">
	import MailProviderSetupGuideItem from './mail-provider-setup-guide-item.svelte';
	import type { MailProviderSetupGuide } from './mail-provider-setup-guides';

	type Props = {
		guide: MailProviderSetupGuide;
	};

	let { guide }: Props = $props();
</script>

<div id={guide.panelID} class="rounded-md border bg-muted/30 p-3 text-sm">
	<ol class="grid gap-4">
		{#each guide.items as item, index}
			<MailProviderSetupGuideItem {item} position={index + 1} isNumbered={guide.isNumbered} />
		{/each}
	</ol>
</div>
