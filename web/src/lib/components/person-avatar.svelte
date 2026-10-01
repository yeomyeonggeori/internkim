<script lang="ts">
	import GradientAvatar from '$lib/components/gradient-avatar.svelte';
	import { personAvatarSeed } from '$lib/person-avatar-seed';
	import { displayPersonName } from '$lib/person-name.svelte';
	import { personPicture } from '$lib/stores/person-picture.svelte';
	import * as Avatar from '$lib/components/ui/avatar';
	import { cn } from '$lib/utils';

	let {
		name = '',
		email = '',
		seed = '',
		image = '',
		memberID = '',
		externalID = '',
		isOnline,
		presenceLabel = '',
		class: className
	}: {
		name?: string;
		email?: string;
		seed?: string;
		image?: string;
		memberID?: string;
		externalID?: string;
		isOnline?: boolean;
		presenceLabel?: string;
		class?: string;
	} = $props();

	const avatarSeed = $derived(personAvatarSeed(email, seed, name));
	const avatarLabel = $derived(displayPersonName(name) || email || 'Person');
	const identity = $derived({ memberID, email, externalID });
	const drawn = $derived(personPicture.pictureOf(identity) || image);

	$effect(() => {
		if (memberID || email || externalID) void personPicture.remember([identity]);
	});

</script>

{#snippet avatar()}
	<Avatar.Root class={cn('size-8 overflow-hidden rounded-full bg-background', className)}>
		{#if drawn}
			<Avatar.Image src={drawn} alt={avatarLabel} />
		{/if}
		<Avatar.Fallback class="relative size-full rounded-[inherit] p-0">
			<GradientAvatar seed={avatarSeed} class="size-full rounded-[inherit]" />
		</Avatar.Fallback>
	</Avatar.Root>
{/snippet}

{#if isOnline === undefined}
	{@render avatar()}
{:else}
	<span class="relative inline-flex shrink-0">
		{@render avatar()}
		<span
			data-presence={isOnline ? 'online' : 'offline'}
			class={cn(
				'absolute -right-px -bottom-px size-[38%] rounded-full ring-[1.5px] ring-background',
				isOnline ? 'bg-success' : 'bg-muted-foreground'
			)}
		></span>
		<span class="sr-only">{presenceLabel}</span>
	</span>
{/if}
