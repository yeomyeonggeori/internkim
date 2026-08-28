<script lang="ts">
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import { cn } from '$lib/utils';

	let {
		name,
		email = '',
		seed,
		image = '',
		class: className
	}: {
		name: string;
		email?: string;
		seed: string;
		image?: string;
		class?: string;
	} = $props();
</script>

<span class={cn('inline-flex min-w-0 max-w-28 items-center gap-1.5', className)}>
	<PersonAvatar {name} {email} {seed} {image} class="size-4 ring-1 ring-border/70" />
	<span class="truncate">{name}</span>
</span>
