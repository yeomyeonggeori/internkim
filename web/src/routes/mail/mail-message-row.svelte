<script lang="ts">
	import { Skeleton } from '$lib/components/ui/skeleton';
	import { mailMessageTimeLabel } from './mail-message-utils';
	import type { MailMessage } from './mail-types';
	import type { mailText } from './text';

	type Props = {
		message?: MailMessage;
		isPlaceholder?: boolean;
		isActive?: boolean;
		isInteractive?: boolean;
		text: (typeof mailText)['ko'];
		selectMessage?: (message: MailMessage) => void;
	};

	let { message, isPlaceholder = false, isActive = false, isInteractive = true, text, selectMessage }: Props = $props();

	const rowClass =
		'w-full rounded-lg border bg-background p-3 text-left shadow-xs transition-shadow data-[active=true]:bg-background data-[active=true]:shadow-md';
	const timeLabel = $derived(message ? mailMessageTimeLabel(message.date, text.yesterday) : '');
</script>

{#snippet placeholderContent()}
	<div class="flex items-center gap-2">
		<span class="size-2 shrink-0 rounded-full bg-muted"></span>
		<Skeleton class="h-4 w-40" />
		<Skeleton class="ml-auto h-3 w-10" />
	</div>
	<Skeleton class="mt-2 ml-4 h-4 w-3/5" />
	<Skeleton class="mt-2 ml-4 h-3 w-4/5" />
{/snippet}

{#snippet rowContent()}
	{#if !message}
		{@render placeholderContent()}
	{:else}
		<div class="flex items-center gap-2">
			<span class="size-2 shrink-0 rounded-full {message.isRead ? 'bg-transparent' : 'bg-primary'}" aria-label={message.isRead ? '' : text.unread}></span>
			<span class="min-w-0 flex-1 truncate text-sm {message.isRead ? 'text-muted-foreground' : 'font-semibold'}">{message.from || text.unknownSender}</span>
			{#if timeLabel}
				<span class="shrink-0 text-xs tabular-nums text-muted-foreground">{timeLabel}</span>
			{/if}
		</div>
		<p class="mt-1 truncate pl-4 text-sm {message.isRead ? '' : 'font-medium'}">{message.subject || text.noSubject}</p>
		{#if message.preview}
			<p class="mt-1 line-clamp-2 pl-4 text-xs leading-5 text-muted-foreground">{message.preview}</p>
		{/if}
	{/if}
{/snippet}

{#if isPlaceholder || !message}
	<div class="{rowClass} pointer-events-none" aria-hidden="true">
		{@render rowContent()}
	</div>
{:else if isInteractive}
	<button type="button" class="{rowClass} hover:bg-muted/60" data-active={isActive} onclick={() => selectMessage?.(message)}>
		{@render rowContent()}
	</button>
{:else}
	<div class={rowClass} data-active={isActive}>
		{@render rowContent()}
	</div>
{/if}
