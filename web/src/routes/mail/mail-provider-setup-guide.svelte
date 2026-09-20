<script lang="ts">
	import MailProviderSetupGuideItem from './mail-provider-setup-guide-item.svelte';
	import type { MailProviderSetupGuide } from './mail-provider-setup-guides';

	type Props = {
		guide: MailProviderSetupGuide;
	};

	let { guide }: Props = $props();
</script>

<ol class="grid gap-4 text-sm">
	{#each guide.items as item, index}
		<MailProviderSetupGuideItem {item} position={index + 1} isNumbered={guide.isNumbered} />
	{/each}
</ol>
