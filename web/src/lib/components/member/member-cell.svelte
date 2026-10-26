<script lang="ts">
	import type { Snippet } from 'svelte';
	import { displayPersonName } from '$lib/person-name.svelte';
	import { cn } from '$lib/utils';
	import MemberAvatar from './member-avatar.svelte';
	import MemberContactActions from './member-contact-actions.svelte';
	import type { MemberProfile } from './member-role';

	let {
		member,
		showsTeam = false,
		isSelected = false,
		onSelect,
		onMessage,
		detail,
		testID,
		selectTestID,
		class: className
	}: {
		member: MemberProfile;
		showsTeam?: boolean;
		isSelected?: boolean;
		onSelect?: () => void;
		onMessage?: () => void;
		detail?: Snippet;
		testID?: string;
		selectTestID?: string;
		class?: string;
	} = $props();
</script>

{#snippet identity()}
	<MemberAvatar {member} role={member.roles[0]} class="size-9" />
	<span class="grid min-w-0 gap-0.5 text-left">
		<span class="flex min-w-0 flex-wrap items-baseline gap-x-1.5 leading-snug">
			<span class="max-w-full truncate text-sm font-medium">{displayPersonName(member.name || member.email)}</span>
			{#if member.jobTitle}<span class="text-muted-foreground max-w-full truncate text-sm">{member.jobTitle}</span>{/if}
			{#if showsTeam && member.team}<span class="text-muted-foreground/70 max-w-full truncate text-xs">{member.team}</span>{/if}
		</span>
		{@render detail?.()}
	</span>
{/snippet}

<div
	class={cn('hover:bg-accent/40 flex min-w-0 items-center gap-2 rounded-lg px-2 py-1.5', isSelected && 'bg-accent', className)}
	data-testid={testID}
>
	{#if onSelect}
		<button type="button" class="flex min-h-11 min-w-0 flex-1 items-center gap-3" onclick={onSelect} data-testid={selectTestID}>
			{@render identity()}
		</button>
	{:else}
		<span class="flex min-h-11 min-w-0 flex-1 items-center gap-3" data-testid={selectTestID}>{@render identity()}</span>
	{/if}
	<MemberContactActions {member} layout="inline" {onMessage} />
</div>
