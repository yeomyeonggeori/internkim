<script lang="ts">
	import { Skeleton } from '$lib/components/ui/skeleton';
	import { mailMessageTimeLabel, mailSenderName } from './mail-message-utils';
	import type { MailMessage } from './mail-types';
	import type { mailText } from './text';
	import type { PageText } from '$lib/i18n/page-text.svelte';

	type Props = {
		message?: MailMessage;
		isPlaceholder?: boolean;
		isActive?: boolean;
		isInteractive?: boolean;
		text: PageText<typeof mailText>;
		selectMessage?: (message: MailMessage) => void;
	};

	let { message, isPlaceholder = false, isActive = false, isInteractive = true, text, selectMessage }: Props = $props();

	const rowClass =
		'flex w-full flex-col items-start gap-2 rounded-lg border p-3 text-left text-sm transition-all data-[active=true]:bg-muted';
	const timeLabel = $derived(message ? mailMessageTimeLabel(message.date, text.yesterday) : '');
</script>

{#snippet placeholderContent()}
	<div class="flex w-full flex-col gap-1">
		<div class="flex h-5 items-center gap-2">
			<Skeleton class="h-4 w-40" />
			<Skeleton class="ml-auto h-3 w-10" />
		</div>
		<div class="flex h-4 items-center"><Skeleton class="h-3 w-3/5" /></div>
	</div>
	<div class="flex h-4 w-full items-center"><Skeleton class="h-3 w-4/5" /></div>
{/snippet}

{#snippet rowContent()}
	{#if !message}
		{@render placeholderContent()}
	{:else}
		<div class="flex w-full flex-col gap-1">
			<div class="flex items-center">
				<div class="flex min-w-0 items-center gap-2">
					<span class="truncate font-semibold">{mailSenderName(message.from) || text.unknownSender}</span>
					{#if !message.isRead}
						<span class="flex size-2 shrink-0 rounded-full bg-sky-500" aria-label={text.unread}></span>
					{/if}
				</div>
				{#if timeLabel}
					<span class="ml-auto shrink-0 pl-2 text-xs tabular-nums {isActive ? 'text-foreground' : 'text-muted-foreground'}">{timeLabel}</span>
				{/if}
			</div>
			<div class="truncate text-xs font-medium">{message.subject || text.noSubject}</div>
		</div>
		{#if message.preview}
			<div class="line-clamp-2 text-xs text-muted-foreground">{message.preview}</div>
		{/if}
	{/if}
{/snippet}

{#if isPlaceholder || !message}
	<div class="{rowClass} pointer-events-none" aria-hidden="true">
		{@render rowContent()}
	</div>
{:else if isInteractive}
	<button type="button" class="{rowClass} hover:bg-accent" data-active={isActive} onclick={() => selectMessage?.(message)}>
		{@render rowContent()}
	</button>
{:else}
	<div class={rowClass} data-active={isActive}>
		{@render rowContent()}
	</div>
{/if}
