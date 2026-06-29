<script lang="ts">
	import type { MailProviderSetupGuideItem } from './mail-provider-setup-guides';

	type Props = {
		item: MailProviderSetupGuideItem;
		position: number;
		isNumbered: boolean;
	};

	let { item, position, isNumbered }: Props = $props();
</script>

<li class="grid gap-2">
	<a class="font-medium text-primary underline-offset-4 hover:underline" href={item.href} target="_blank" rel="noopener noreferrer">
		{#if isNumbered}{position}. {/if}{item.title}
	</a>
	{#each item.notes as note}
		<p class="text-xs leading-5 text-muted-foreground">{note}</p>
	{/each}
	<img
		class="mx-auto w-full max-w-sm rounded-md border bg-background"
		src={item.image.src}
		alt={item.image.alt}
	/>
</li>
