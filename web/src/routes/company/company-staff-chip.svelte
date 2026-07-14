<script lang="ts">
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import { cn } from '$lib/utils';
	import type { CompanyShareMember } from './company-page-model';

	let {
		member,
		fallbackName,
		defaultJobTitle,
		compact = false
	}: {
		member: CompanyShareMember;
		fallbackName: string;
		defaultJobTitle: string;
		compact?: boolean;
	} = $props();

	const surname = $derived(member.surname || fallbackName);
	const jobTitle = $derived(member.jobTitle || defaultJobTitle);
	const label = $derived(`${surname} ${jobTitle}`);
</script>

<span class={cn('inline-flex min-w-0 items-center rounded-full border border-border/70 bg-background', compact ? 'max-w-36 gap-1 px-1.5 py-0.5' : 'gap-2 py-1 pr-2.5 pl-1')}>
	<PersonAvatar seed={member.seed} name={label} image={member.image ?? ''} class={compact ? 'size-4' : 'size-7'} />
	<span class={cn('truncate font-medium', compact ? 'text-[0.6875rem]' : 'text-xs')}>{surname}</span>
	<span class={cn('text-muted-foreground truncate', compact ? 'text-[0.6875rem]' : 'text-xs')}>{jobTitle}</span>
</span>
