<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import * as ButtonGroup from '$lib/components/ui/button-group';
	import MailIcon from '@lucide/svelte/icons/mail';
	import MessageSquareIcon from '@lucide/svelte/icons/message-square';
	import PhoneIcon from '@lucide/svelte/icons/phone';
	import type { organizationDirectoryText } from './text';

	let {
		email,
		phoneNumber = '',
		text
	}: {
		email: string;
		phoneNumber?: string;
		text: typeof organizationDirectoryText.ko;
	} = $props();
</script>

<ButtonGroup.Root class="shrink-0">
	<Button variant="ghost" size="icon" href={`mailto:${email}`} aria-label={text.sendEmail} title={email}>
		<MailIcon />
	</Button>
	<Button variant="ghost" size="icon" disabled aria-label={text.sendDirectMessage}>
		<MessageSquareIcon />
	</Button>
	{#if phoneNumber}
		<Button variant="ghost" size="icon" href={`tel:${phoneNumber}`} aria-label={text.callPhone} title={phoneNumber}>
			<PhoneIcon />
		</Button>
	{/if}
</ButtonGroup.Root>
