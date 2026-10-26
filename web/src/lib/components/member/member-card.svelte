<script lang="ts">
	import TimerIcon from '@lucide/svelte/icons/timer';
	import { Badge } from '$lib/components/ui/badge';
	import * as Tooltip from '$lib/components/ui/tooltip';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { displayPersonName } from '$lib/person-name.svelte';
	import { cn } from '$lib/utils';
	import MemberAvatar from './member-avatar.svelte';
	import MemberContactActions from './member-contact-actions.svelte';
	import { memberRoleLook, type MemberProfile } from './member-role';
	import { memberText } from './member-text';

	let {
		member,
		tenure,
		showsTeam = false,
		isSelected = false,
		onSelect,
		onMessage,
		testID,
		selectTestID,
		class: className
	}: {
		member: MemberProfile;
		tenure?: { label: string; detail: string };
		showsTeam?: boolean;
		isSelected?: boolean;
		onSelect?: () => void;
		onMessage?: () => void;
		testID?: string;
		selectTestID?: string;
		class?: string;
	} = $props();

	const text = createPageText(memberText);
</script>

{#snippet identity()}
	<MemberAvatar {member} class="size-16" />
	<span class="grid w-full min-w-0 gap-0.5">
		<span class="truncate text-sm font-medium">{displayPersonName(member.name || member.email)}</span>
		{#if member.jobTitle}<span class="text-muted-foreground truncate text-xs">{member.jobTitle}</span>{/if}
		{#if showsTeam && member.team}<span class="text-muted-foreground/70 truncate text-xs">{member.team}</span>{/if}
	</span>
{/snippet}

<div
	class={cn('bg-card hover:bg-accent/40 relative flex min-w-0 flex-col items-center overflow-hidden rounded-xl border text-center', isSelected && 'shadow-lg', className)}
	data-testid={testID}
>
	<div class="absolute inset-x-2 top-2 z-10 flex items-start justify-between gap-1">
		<span class="grid justify-items-start gap-1">
			{#each member.roles as role (role)}
				{@const look = memberRoleLook[role]}
				<Badge variant="secondary" class={look.class}><look.icon />{text[role]}</Badge>
			{/each}
		</span>
		{#if tenure}
			<Tooltip.Root>
				<Tooltip.Trigger>
					{#snippet child({ props })}
						<Badge {...props} variant="outline" class="bg-background h-5 gap-1 rounded-full px-1.5 font-normal tabular-nums">
							<TimerIcon class="size-3" />
							{tenure.label}
						</Badge>
					{/snippet}
				</Tooltip.Trigger>
				<Tooltip.Content side="top">{tenure.detail}</Tooltip.Content>
			</Tooltip.Root>
		{/if}
	</div>
	{#if onSelect}
		<button type="button" class="flex w-full min-w-0 flex-col items-center gap-2 px-4 pt-8 pb-3" onclick={onSelect} data-testid={selectTestID}>
			{@render identity()}
		</button>
	{:else}
		<span class="flex w-full min-w-0 flex-col items-center gap-2 px-4 pt-8 pb-3" data-testid={selectTestID}>{@render identity()}</span>
	{/if}
	<MemberContactActions {member} layout="bar" {onMessage} />
</div>
