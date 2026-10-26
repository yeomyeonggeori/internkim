<script lang="ts">
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { displayPersonName } from '$lib/person-name.svelte';
	import { cn } from '$lib/utils';
	import { memberRoleLook, type MemberProfile, type MemberRole } from './member-role';
	import { memberText } from './member-text';

	let {
		member,
		role,
		class: className
	}: { member: MemberProfile; role?: MemberRole; class?: string } = $props();

	const text = createPageText(memberText);
	const look = $derived(role ? memberRoleLook[role] : undefined);
</script>

<span class="relative inline-flex shrink-0">
	{#if role && look}
		<span
			class={cn('absolute -top-2.5 -left-2.5 grid size-5 place-items-center rounded-full', look.class)}
			role="img"
			aria-label={text[role]}
			title={text[role]}
		>
			<look.icon class="size-2.5" strokeWidth={2.5} />
		</span>
	{/if}
	<PersonAvatar
		name={displayPersonName(member.name)}
		email={member.email}
		seed={member.memberID}
		memberID={member.memberID}
		externalID={member.externalID ?? ''}
		image={member.image ?? ''}
		class={cn('relative', role && 'ring-2 ring-card', className)}
	/>
</span>
