<script lang="ts">
	import Identicon from '$lib/components/identicon.svelte';
	import { personAvatarSeed } from '$lib/person-avatar-seed';
	import * as Avatar from '$lib/components/ui/avatar';
	import { cn } from '$lib/utils';

	let {
		name = '',
		email = '',
		seed = '',
		image = '',
		class: className
	}: {
		name?: string;
		email?: string;
		seed?: string;
		image?: string;
		class?: string;
	} = $props();

	const avatarSeed = $derived(personAvatarSeed(email, seed, name));
	const avatarLabel = $derived(name || email || 'Person');
</script>

<Avatar.Root class={cn('size-8 overflow-hidden rounded-full bg-background', className)}>
	{#if image}
		<Avatar.Image src={image} alt={avatarLabel} />
	{/if}
	<Avatar.Fallback class="size-full rounded-[inherit] p-0">
		<Identicon seed={avatarSeed} class="size-full rounded-[inherit]" />
	</Avatar.Fallback>
</Avatar.Root>
