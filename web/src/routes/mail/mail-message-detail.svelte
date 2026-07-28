<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import TooltipIconButton from '$lib/components/tooltip-icon-button.svelte';
	import * as ButtonGroup from '$lib/components/ui/button-group';
	import { Separator } from '$lib/components/ui/separator';
	import ArchiveIcon from '@lucide/svelte/icons/archive';
	import ArrowLeftIcon from '@lucide/svelte/icons/arrow-left';
	import MailIcon from '@lucide/svelte/icons/mail';
	import PencilIcon from '@lucide/svelte/icons/pencil';
	import ForwardIcon from '@lucide/svelte/icons/forward';
	import ReplyIcon from '@lucide/svelte/icons/reply';
	import SettingsIcon from '@lucide/svelte/icons/settings';
	import Trash2Icon from '@lucide/svelte/icons/trash-2';
	import { MAIL_MESSAGE_IFRAME_SANDBOX, mailHTMLDocument } from './mail-message-utils';
	import type { MailAccount, MailMessage } from './mail-types';
	import type { mailText } from './text';

	type Props = {
		account: MailAccount;
		selectedMessage: MailMessage | null;
		hasLoadedAccount: boolean;
		isLoadingMessage: boolean;
		messageBody: string;
		messageBodyHTML: string;
		text: (typeof mailText)['ko'];
		moveSelectedMessage: (targetHint: string) => void | Promise<void>;
		openReply: () => void;
		openForward: () => void;
		openCompose: () => void;
		openSettings: () => void;
		goBack?: () => void;
	};

	let {
		account,
		selectedMessage,
		hasLoadedAccount,
		isLoadingMessage,
		messageBody,
		messageBodyHTML,
		text,
		moveSelectedMessage,
		openReply,
		openForward,
		openCompose,
		openSettings,
		goBack
	}: Props = $props();
</script>

<section class="flex min-h-0 flex-1 flex-col bg-background">
	<header class="flex h-14 shrink-0 items-center gap-2 border-b px-3">
		{#if goBack}
			<TooltipIconButton label={text.backToList} variant="ghost" size="icon-sm" onclick={goBack}>
				<ArrowLeftIcon />
			</TooltipIconButton>
		{/if}
		<div class="flex flex-1 items-center justify-end gap-2">
			<ButtonGroup.Root>
				<TooltipIconButton label={text.archive} variant="outline" size="icon-sm" onclick={() => moveSelectedMessage('archive')} disabled={!selectedMessage}>
					<ArchiveIcon />
				</TooltipIconButton>
				<TooltipIconButton label={text.trash} variant="outline" size="icon-sm" onclick={() => moveSelectedMessage('trash')} disabled={!selectedMessage}>
					<Trash2Icon />
				</TooltipIconButton>
			</ButtonGroup.Root>
			<ButtonGroup.Root>
				<TooltipIconButton label={text.reply} variant="outline" size="icon-sm" onclick={openReply} disabled={!selectedMessage}>
					<ReplyIcon />
				</TooltipIconButton>
				<TooltipIconButton label={text.forward} variant="outline" size="icon-sm" onclick={openForward} disabled={!selectedMessage}>
					<ForwardIcon />
				</TooltipIconButton>
			</ButtonGroup.Root>
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
				{#if isLoadingMessage && !messageBodyHTML && !messageBody}
					<p class="text-sm text-muted-foreground">{text.loadingMessage}</p>
				{:else}
					{#if isLoadingMessage}
						<p class="mb-3 text-xs text-muted-foreground">{text.loadingMessage}</p>
					{/if}
					{#if messageBodyHTML}
						<iframe
							title={text.messageBody}
							class="min-h-[62vh] w-full rounded-md border bg-white"
							sandbox={MAIL_MESSAGE_IFRAME_SANDBOX}
							referrerpolicy="no-referrer"
							srcdoc={mailHTMLDocument(messageBodyHTML)}
						></iframe>
					{:else}
						<p class="whitespace-pre-wrap text-sm leading-6">{messageBody}</p>
					{/if}
				{/if}
			</div>
		{:else}
			<div class="flex h-full items-center justify-center">
				<div class="max-w-sm text-center">
					<div class="mx-auto flex size-12 items-center justify-center rounded-xl border bg-muted/50">
						<MailIcon class="size-5 text-muted-foreground" />
					</div>
					<h2 class="mt-4 text-lg font-semibold">{hasLoadedAccount ? (account.isConfigured ? text.ready : text.notConnected) : text.checkingMail}</h2>
					<p class="mt-2 text-sm leading-6 text-muted-foreground">{hasLoadedAccount ? (account.isConfigured ? text.chooseMessage : text.connectDescription) : text.checkingMailDescription}</p>
					<Button class="mt-4 gap-2" variant="secondary" onclick={() => (account.isConfigured ? openCompose() : openSettings())} disabled={!hasLoadedAccount}>
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
</section>
