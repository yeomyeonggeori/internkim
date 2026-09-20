<script lang="ts">
	import TooltipIconButton from '$lib/components/tooltip-icon-button.svelte';
	import * as Avatar from '$lib/components/ui/avatar';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu';
	import { Separator } from '$lib/components/ui/separator';
	import ArchiveIcon from '@lucide/svelte/icons/archive';
	import ArchiveXIcon from '@lucide/svelte/icons/archive-x';
	import ArrowLeftIcon from '@lucide/svelte/icons/arrow-left';
	import ForwardIcon from '@lucide/svelte/icons/forward';
	import MailIcon from '@lucide/svelte/icons/mail';
	import MoreVerticalIcon from '@lucide/svelte/icons/more-vertical';
	import ReplyIcon from '@lucide/svelte/icons/reply';
	import Trash2Icon from '@lucide/svelte/icons/trash-2';

	import { MAIL_MESSAGE_IFRAME_SANDBOX, mailHTMLDocument, mailSenderAddress, mailSenderName } from './mail-message-utils';
	import type { MailMoveTarget } from './mail-page-utils';
	import type { MailMessage } from './mail-types';
	import type { mailText } from './text';
	import type { PageText } from '$lib/i18n/page-text.svelte';

	const MAIL_MOVE_TARGET_ICONS = { archive: ArchiveIcon, junk: ArchiveXIcon, trash: Trash2Icon };

	type Props = {
		selectedMessage: MailMessage | null;
		hasVisibleMessages: boolean;
		isLoadingMessage: boolean;
		messageBody: string;
		messageBodyHTML: string;
		moveTargets: MailMoveTarget[];
		text: PageText<typeof mailText>;
		moveSelectedMessage: (target: MailMoveTarget) => void | Promise<void>;
		markSelectedMessageUnread: () => void | Promise<void>;
		openReply: () => void;
		openForward: () => void;
		goBack?: () => void;
	};

	let {
		selectedMessage,
		hasVisibleMessages,
		isLoadingMessage,
		messageBody,
		messageBodyHTML,
		moveTargets,
		text,
		moveSelectedMessage,
		markSelectedMessageUnread,
		openReply,
		openForward,
		goBack
	}: Props = $props();

	const senderName = $derived(mailSenderName(selectedMessage?.from ?? ''));
	const senderAddress = $derived(mailSenderAddress(selectedMessage?.from ?? ''));
	const senderInitials = $derived(
		senderName
			.split(/[\s@.]+/)
			.filter(Boolean)
			.slice(0, 2)
			.map((chunk) => chunk[0].toUpperCase())
			.join('') || '?'
	);
	const dateLabel = $derived(
		selectedMessage?.date ? new Date(selectedMessage.date).toLocaleString(undefined, { dateStyle: 'medium', timeStyle: 'short' }) : ''
	);
</script>

<section class="flex min-h-0 flex-1 flex-col bg-background">
	<header class="flex h-[52px] shrink-0 items-center gap-2 border-b px-2">
		{#if goBack}
			<TooltipIconButton label={text.backToList} variant="ghost" size="icon-sm" onclick={goBack}>
				<ArrowLeftIcon />
			</TooltipIconButton>
		{/if}
		{#if selectedMessage}
			<div class="flex items-center gap-1">
				{#each moveTargets as target (target)}
					{@const Icon = MAIL_MOVE_TARGET_ICONS[target]}
					<TooltipIconButton label={text.moveTargets[target]} variant="ghost" size="icon-sm" onclick={() => moveSelectedMessage(target)}>
						<Icon />
					</TooltipIconButton>
				{/each}
			</div>
			<div class="ml-auto flex items-center gap-1">
				<TooltipIconButton label={text.reply} variant="ghost" size="icon-sm" onclick={openReply}>
					<ReplyIcon />
				</TooltipIconButton>
				<TooltipIconButton label={text.forward} variant="ghost" size="icon-sm" onclick={openForward}>
					<ForwardIcon />
				</TooltipIconButton>
				<Separator orientation="vertical" class="mx-1 !h-6" />
				<DropdownMenu.Root>
					<DropdownMenu.Trigger>
						{#snippet child({ props })}
							<TooltipIconButton label={text.moreActions} variant="ghost" size="icon-sm" {...props}>
								<MoreVerticalIcon />
							</TooltipIconButton>
						{/snippet}
					</DropdownMenu.Trigger>
					<DropdownMenu.Content align="end" class="w-48">
						<DropdownMenu.Item disabled={!selectedMessage.isRead} onSelect={markSelectedMessageUnread}>
							<MailIcon />
							{text.markUnread}
						</DropdownMenu.Item>
					</DropdownMenu.Content>
				</DropdownMenu.Root>
			</div>
		{/if}
	</header>

	{#if selectedMessage}
		<div class="flex min-h-0 flex-1 flex-col">
			<div class="flex shrink-0 items-start gap-4 p-4 text-sm">
				<Avatar.Root>
					<Avatar.Fallback>{senderInitials}</Avatar.Fallback>
				</Avatar.Root>
				<div class="grid min-w-0 gap-1">
					<div class="truncate font-semibold">{senderName || text.unknownSender}</div>
					<div class="line-clamp-1 text-xs">{selectedMessage.subject || text.noSubject}</div>
					{#if senderAddress}
						<div class="line-clamp-1 text-xs"><span class="font-medium">{text.from}:</span> {senderAddress}</div>
					{/if}
					{#if selectedMessage.to}
						<div class="line-clamp-1 text-xs text-muted-foreground"><span class="font-medium">{text.to}:</span> {selectedMessage.to}</div>
					{/if}
				</div>
				{#if dateLabel}
					<div class="ml-auto shrink-0 pl-2 text-xs text-muted-foreground">{dateLabel}</div>
				{/if}
			</div>
			<Separator />
			<div class="min-h-0 flex-1 overflow-auto p-4 text-sm">
				{#if isLoadingMessage && !messageBodyHTML && !messageBody}
					<p class="text-muted-foreground">{text.loadingMessage}</p>
				{:else}
					{#if isLoadingMessage}
						<p class="mb-3 text-xs text-muted-foreground">{text.loadingMessage}</p>
					{/if}
					{#if messageBodyHTML}
						<iframe
							title={text.messageBody}
							class="h-full min-h-[62vh] w-full rounded-md border bg-white"
							sandbox={MAIL_MESSAGE_IFRAME_SANDBOX}
							referrerpolicy="no-referrer"
							srcdoc={mailHTMLDocument(messageBodyHTML)}
						></iframe>
					{:else}
						<p class="whitespace-pre-wrap leading-6">{messageBody}</p>
					{/if}
				{/if}
			</div>
		</div>
	{:else if hasVisibleMessages}
		<div class="p-8 text-center text-sm text-muted-foreground">{text.chooseMessage}</div>
	{/if}
</section>
