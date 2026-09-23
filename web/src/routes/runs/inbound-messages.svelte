<script lang="ts">
	import { page } from '$app/state';
	import { taskRunDetailPathOf } from '$lib/app-shell';
	import { Badge } from '$lib/components/ui/badge';
	import * as Card from '$lib/components/ui/card';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import * as Table from '$lib/components/ui/table';
	import { onMount } from 'svelte';
	import DecisionTable from './decision-table.svelte';
	import { decisionRows, fetchInboundMessages, messageKeyOf, type InboundMessage, type InboundOutcome } from './llm-calls';
	import { glyphOfEmojiName } from '$lib/messenger/emoji-glyph';
	import { formatTaskTimestamp } from './runs-view';
	import type { TasksText } from './text';

	let { text }: { text: TasksText } = $props();

	const inboundMessageLimit = 50;
	let inboundMessages = $state<InboundMessage[]>([]);
	let isLoading = $state(true);
	let loadError = $state('');
	let openMessageID = $state('');

	async function loadInboundMessages() {
		isLoading = true;
		loadError = '';
		try {
			inboundMessages = await fetchInboundMessages(inboundMessageLimit);
		} catch {
			loadError = text.inboundLoadError;
		} finally {
			isLoading = false;
		}
	}

	function outcomeLabel(outcome: InboundOutcome): string {
		switch (outcome) {
			case 'task':
				return text.outcomeTask;
			case 'reacted':
				return text.outcomeReacted;
			case 'ignored':
				return text.outcomeIgnored;
			case 'failed':
				return text.outcomeFailed;
			default:
				return text.outcomePending;
		}
	}

	function outcomeVariant(outcome: InboundOutcome) {
		if (outcome === 'task') return 'default';
		if (outcome === 'failed') return 'destructive';
		if (outcome === 'reacted') return 'secondary';
		return 'outline';
	}

	function reactionLabel(inboundMessage: InboundMessage): string {
		if (inboundMessage.reactionProbability === undefined) return text.reactionNotDecided;
		return text.reactionChance
			.replace('{probability}', inboundMessage.reactionProbability.toFixed(2))
			.replace('{draw}', inboundMessage.reactionDraw?.toFixed(2) ?? '—');
	}

	function reactionGlyph(emojiName: string): string {
		return glyphOfEmojiName(emojiName) ?? `:${emojiName}:`;
	}

	function toggleDecision(messageID: string) {
		openMessageID = openMessageID === messageID ? '' : messageID;
	}

	function decisionRowsOf(inboundMessage: InboundMessage) {
		if (!inboundMessage.decision) return [];
		const messageKey = messageKeyOf(inboundMessage.decision, inboundMessage.messageID);
		return messageKey ? decisionRows(inboundMessage.decision, messageKey) : [];
	}

	onMount(() => void loadInboundMessages());
</script>

{#snippet outcomeBadge(inboundMessage: InboundMessage)}
	{#if inboundMessage.taskRunID}
		<a href={taskRunDetailPathOf(page.url.pathname, inboundMessage.taskRunID)} onclick={(event) => event.stopPropagation()}>
			<Badge variant={outcomeVariant(inboundMessage.outcome)}>{outcomeLabel(inboundMessage.outcome)}</Badge>
		</a>
	{:else}
		<Badge variant={outcomeVariant(inboundMessage.outcome)} title={inboundMessage.ignoreReason}>
			{inboundMessage.reactionEmoji && inboundMessage.outcome === 'reacted' ? `${outcomeLabel(inboundMessage.outcome)} ${reactionGlyph(inboundMessage.reactionEmoji)}` : outcomeLabel(inboundMessage.outcome)}
		</Badge>
	{/if}
{/snippet}

<p class="text-sm text-muted-foreground">{text.inboundDescription}</p>

{#if loadError}
	<Card.Root size="sm" class="border-destructive/30">
		<Card.Content class="text-sm text-destructive">{loadError}</Card.Content>
	</Card.Root>
{:else if isLoading}
	<Card.Root>
		<Card.Content class="flex flex-col gap-2">
			<Skeleton class="h-10 w-full" />
			<Skeleton class="h-10 w-full" />
			<Skeleton class="h-10 w-full" />
		</Card.Content>
	</Card.Root>
{:else if inboundMessages.length === 0}
	<Card.Root size="sm">
		<Card.Content class="text-sm text-muted-foreground">{text.inboundEmpty}</Card.Content>
	</Card.Root>
{:else}
	<Card.Root class="min-w-0 max-md:mb-[calc(5rem+env(safe-area-inset-bottom))]">
		<Card.Content class="px-0">
			<div class="divide-y md:hidden">
				{#each inboundMessages as inboundMessage (inboundMessage.messageID)}
					{@const rows = decisionRowsOf(inboundMessage)}
					<div class="flex flex-col gap-2 px-4 py-3">
						<div class="flex items-center justify-between gap-3 text-xs text-muted-foreground">
							<span class="truncate font-medium text-foreground">{inboundMessage.senderName || '—'}</span>
							<span class="shrink-0">{formatTaskTimestamp(inboundMessage.ingestedAt)}</span>
						</div>
						<p class="line-clamp-2 text-sm">{inboundMessage.promptPreview || '—'}</p>
						<div class="flex items-center justify-between gap-3">
							{@render outcomeBadge(inboundMessage)}
							{#if rows.length > 0}
								<button type="button" class="min-h-11 text-xs text-muted-foreground tabular-nums" aria-expanded={openMessageID === inboundMessage.messageID} onclick={() => toggleDecision(inboundMessage.messageID)}>
									{reactionLabel(inboundMessage)}
								</button>
							{:else}
								<span class="text-xs text-muted-foreground">{reactionLabel(inboundMessage)}</span>
							{/if}
						</div>
						{#if openMessageID === inboundMessage.messageID}
							<div class="overflow-x-auto rounded-md border">
								<DecisionTable {rows} {text} />
							</div>
						{/if}
					</div>
				{/each}
			</div>
			<Table.Root class="hidden md:table">
				<Table.Header>
					<Table.Row>
						<Table.Head class="w-36">{text.columnReceived}</Table.Head>
						<Table.Head class="w-32">{text.columnSender}</Table.Head>
						<Table.Head>{text.columnMessage}</Table.Head>
						<Table.Head class="w-24">{text.columnOutcome}</Table.Head>
						<Table.Head class="w-44 text-right">{text.columnReaction}</Table.Head>
					</Table.Row>
				</Table.Header>
				<Table.Body>
					{#each inboundMessages as inboundMessage (inboundMessage.messageID)}
						{@const rows = decisionRowsOf(inboundMessage)}
						<Table.Row
							class={rows.length > 0 ? 'cursor-pointer hover:bg-muted/50' : ''}
							aria-expanded={rows.length > 0 ? openMessageID === inboundMessage.messageID : undefined}
							onclick={() => rows.length > 0 && toggleDecision(inboundMessage.messageID)}
						>
							<Table.Cell class="text-xs whitespace-nowrap text-muted-foreground">{formatTaskTimestamp(inboundMessage.ingestedAt)}</Table.Cell>
							<Table.Cell class="truncate text-sm">{inboundMessage.senderName || '—'}</Table.Cell>
							<Table.Cell class="max-w-0">
								<p class="truncate text-sm">{inboundMessage.promptPreview || '—'}</p>
							</Table.Cell>
							<Table.Cell>{@render outcomeBadge(inboundMessage)}</Table.Cell>
							<Table.Cell class="text-right text-xs whitespace-nowrap text-muted-foreground tabular-nums">{reactionLabel(inboundMessage)}</Table.Cell>
						</Table.Row>
						{#if openMessageID === inboundMessage.messageID}
							<Table.Row class="hover:bg-transparent">
								<Table.Cell colspan={5} class="bg-muted/20">
									<DecisionTable {rows} {text} />
								</Table.Cell>
							</Table.Row>
						{/if}
					{/each}
				</Table.Body>
			</Table.Root>
		</Card.Content>
	</Card.Root>
{/if}
