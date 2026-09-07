<script lang="ts">
	import GradientAvatar from '$lib/components/gradient-avatar.svelte';
	import * as Avatar from '$lib/components/ui/avatar';
	import { cn } from '$lib/utils';

	let {
		name = '',
		image = '',
		class: className
	}: { name?: string; image?: string; class?: string } = $props();

	const seed = $derived(name.trim() || '?');
</script>

<Avatar.Root class={cn('size-8 overflow-hidden rounded-md bg-background', className)}>
	{#if image}
		<Avatar.Image src={image} alt={name} class="object-cover" />
	{/if}
	<Avatar.Fallback class="relative size-full rounded-[inherit] p-0">
		<GradientAvatar {seed} class="size-full rounded-[inherit]" />
	</Avatar.Fallback>
</Avatar.Root>
