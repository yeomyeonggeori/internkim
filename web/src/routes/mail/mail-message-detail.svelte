<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import { Separator } from '$lib/components/ui/separator';
	import ArchiveIcon from '@lucide/svelte/icons/archive';
	import MailIcon from '@lucide/svelte/icons/mail';
	import MailOpenIcon from '@lucide/svelte/icons/mail-open';
	import PencilIcon from '@lucide/svelte/icons/pencil';
	import SendIcon from '@lucide/svelte/icons/send';
	import SettingsIcon from '@lucide/svelte/icons/settings';
	import Trash2Icon from '@lucide/svelte/icons/trash-2';
	import { mailHTMLDocument } from './mail-message-utils';
	import type { MailAccount, MailMessage } from './mail-types';
	import type { mailText } from './text';

	type Props = {
		account: MailAccount;
		selectedMailbox: string;
		selectedMessage: MailMessage | null;
		isLoadingMessage: boolean;
		messageBody: string;
		messageBodyHTML: string;
		text: (typeof mailText)['ko'];
		moveSelectedMessage: (targetHint: string) => void | Promise<void>;
		toggleSelectedMessageRead: () => void | Promise<void>;
		openReply: () => void;
		openCompose: () => void;
		openSettings: () => void;
	};

	let {
		account,
		selectedMailbox,
		selectedMessage,
		isLoadingMessage,
		messageBody,
		messageBodyHTML,
		text,
		moveSelectedMessage,
		toggleSelectedMessageRead,
		openReply,
		openCompose,
		openSettings
	}: Props = $props();
</script>

<section class="flex min-h-0 flex-col bg-background max-lg:hidden">
	<header class="flex h-14 shrink-0 items-center gap-2 border-b px-4">
		<div class="min-w-0 flex-1">
			<p class="truncate text-sm text-muted-foreground">{text.allInboxes} / {selectedMailbox}</p>
			<p class="truncate text-sm font-medium">{selectedMessage?.subject || text.selectMessage}</p>
		</div>
		<div class="flex items-center gap-1">
			<Button variant="ghost" size="icon-sm" aria-label={text.archive} onclick={() => moveSelectedMessage('archive')} disabled={!selectedMessage}>
				<ArchiveIcon />
			</Button>
			<Button variant="ghost" size="icon-sm" aria-label={text.trash} onclick={() => moveSelectedMessage('trash')} disabled={!selectedMessage}>
				<Trash2Icon />
			</Button>
			<Button variant="ghost" size="icon-sm" aria-label={text.markRead} onclick={toggleSelectedMessageRead} disabled={!selectedMessage}>
				{#if selectedMessage?.isRead}
					<MailIcon />
				{:else}
					<MailOpenIcon />
				{/if}
			</Button>
			<Button variant="ghost" size="icon-sm" aria-label={text.reply} onclick={openReply} disabled={!selectedMessage}>
				<SendIcon />
			</Button>
		</div>
	</header>

	<div class="min-h-0 flex-1 overflow-auto p-6">
		{#if selectedMessage}
			<div class="mx-auto max-w-3xl space-y-4">
				<div>
					<p class="text-xs text-muted-foreground">{selectedMessage.from}</p>
					<h2 class="mt-2 text-2xl font-semibold tracking-tight">{selectedMessage.subject || text.noSubject}</h2>
					{#if selectedMessage.to}
						<p class="mt-2 text-xs text-muted-foreground">{text.to} {selectedMessage.to}</p>
					{/if}
				</div>
				<Separator />
				{#if isLoadingMessage}
					<p class="text-sm text-muted-foreground">{text.loadingMessage}</p>
				{:else if messageBodyHTML}
					<iframe
						title={text.messageBody}
						class="min-h-[62vh] w-full rounded-md border bg-white"
						sandbox=""
						referrerpolicy="no-referrer"
						srcdoc={mailHTMLDocument(messageBodyHTML)}
					></iframe>
				{:else}
					<p class="whitespace-pre-wrap text-sm leading-6">{messageBody}</p>
				{/if}
			</div>
		{:else}
			<div class="flex h-full items-center justify-center">
				<div class="max-w-sm text-center">
					<div class="mx-auto flex size-12 items-center justify-center rounded-xl border bg-muted/50">
						<MailIcon class="size-5 text-muted-foreground" />
					</div>
					<h2 class="mt-4 text-lg font-semibold">{account.isConfigured ? text.ready : text.notConnected}</h2>
					<p class="mt-2 text-sm leading-6 text-muted-foreground">{account.isConfigured ? text.chooseMessage : text.connectDescription}</p>
					<Button class="mt-4 gap-2" variant="secondary" onclick={() => (account.isConfigured ? openCompose() : openSettings())}>
						{#if account.isConfigured}
							<PencilIcon />
							{text.compose}
						{:else}
							<SettingsIcon />
							{text.connectAccount}
						{/if}
					</Button>
				</div>
			</div>
		{/if}
	</div>

	<div class="border-t p-4">
		<Button class="w-full justify-start gap-2" variant="outline" onclick={selectedMessage ? openReply : openCompose} disabled={!account.isConfigured}>
			<PencilIcon />
			{selectedMessage ? text.replyAction : text.composeAction}
		</Button>
	</div>
</section>
