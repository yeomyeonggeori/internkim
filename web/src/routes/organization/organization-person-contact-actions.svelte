<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import * as ButtonGroup from '$lib/components/ui/button-group';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu';
	import ChevronDownIcon from '@lucide/svelte/icons/chevron-down';
	import CopyIcon from '@lucide/svelte/icons/copy';
	import MailIcon from '@lucide/svelte/icons/mail';
	import PhoneIcon from '@lucide/svelte/icons/phone';
	import { toast } from 'svelte-sonner';
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

	async function copyEmail(): Promise<void> {
		await navigator.clipboard.writeText(email);
		toast.success(text.emailCopied);
	}

	async function copyPhoneNumber(): Promise<void> {
		await navigator.clipboard.writeText(phoneNumber);
		toast.success(text.phoneNumberCopied);
	}
</script>

<ButtonGroup.Root class="shrink-0">
	<Button variant="outline" size="icon-sm" href={`mailto:${email}`} aria-label={text.sendEmail} title={email}>
		<MailIcon />
	</Button>
	{#if phoneNumber}
		<Button variant="outline" size="icon-sm" href={`tel:${phoneNumber}`} aria-label={text.callPhone} title={phoneNumber}>
			<PhoneIcon />
		</Button>
	{/if}
	<DropdownMenu.Root>
		<DropdownMenu.Trigger>
			{#snippet child({ props })}
				<Button {...props} variant="outline" size="icon-sm" aria-label={text.contactActions}>
					<ChevronDownIcon />
				</Button>
			{/snippet}
		</DropdownMenu.Trigger>
		<DropdownMenu.Content align="end" class="min-w-44">
			<DropdownMenu.Item onSelect={() => void copyEmail()}>
				<CopyIcon />
				{text.copyEmail}
			</DropdownMenu.Item>
			{#if phoneNumber}
				<DropdownMenu.Item onSelect={() => void copyPhoneNumber()}>
					<CopyIcon />
					{text.copyPhoneNumber}
				</DropdownMenu.Item>
			{/if}
		</DropdownMenu.Content>
	</DropdownMenu.Root>
</ButtonGroup.Root>
