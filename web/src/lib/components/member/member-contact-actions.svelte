<script lang="ts">
	import MailIcon from '@lucide/svelte/icons/mail';
	import MessageCircleIcon from '@lucide/svelte/icons/message-circle';
	import PhoneIcon from '@lucide/svelte/icons/phone';
	import { Button } from '$lib/components/ui/button';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { cn } from '$lib/utils';
	import type { MemberProfile } from './member-role';
	import { memberText } from './member-text';

	let {
		member,
		layout,
		onMessage
	}: { member: MemberProfile; layout: 'inline' | 'bar'; onMessage?: () => void } = $props();

	const text = createPageText(memberText);
	const buttonClass = $derived(layout === 'bar' ? 'h-10 flex-1 rounded-none text-muted-foreground' : 'text-muted-foreground');
</script>

<div class={cn('flex shrink-0', layout === 'bar' && 'w-full divide-x border-t')}>
	<Button variant="ghost" size="icon" class={buttonClass} disabled={!onMessage} onclick={onMessage} aria-label={text.sendDirectMessage} title={text.sendDirectMessage}>
		<MessageCircleIcon />
	</Button>
	{#if member.phoneNumber}
		<Button variant="ghost" size="icon" class={buttonClass} href={`tel:${member.phoneNumber}`} aria-label={text.callPhone} title={member.phoneNumber}>
			<PhoneIcon />
		</Button>
	{:else}
		<Button variant="ghost" size="icon" class={buttonClass} disabled aria-label={text.noPhoneNumber} title={text.noPhoneNumber}>
			<PhoneIcon />
		</Button>
	{/if}
	<Button variant="ghost" size="icon" class={buttonClass} href={member.email ? `mailto:${member.email}` : undefined} disabled={!member.email} aria-label={text.sendEmail} title={member.email || text.sendEmail}>
		<MailIcon />
	</Button>
</div>
