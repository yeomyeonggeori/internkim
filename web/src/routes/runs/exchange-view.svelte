<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import { buttonVariants } from '$lib/components/ui/button';
	import * as Collapsible from '$lib/components/ui/collapsible';
	import { CopyButton } from '$lib/components/ui/copy-button';
	import ChevronRightIcon from '@lucide/svelte/icons/chevron-right';
	import type { Exchange, ExchangeMessage } from './llm-calls';
	import type { TasksText } from './text';

	let { exchange, text }: { exchange: Exchange; text: TasksText } = $props();

	const inputDocument = $derived(exchange.input === undefined ? '' : JSON.stringify(exchange.input, undefined, 2));
	const stateDocument = $derived(exchange.decisionState === undefined ? '' : JSON.stringify(exchange.decisionState, undefined, 2));

	function roleLabel(role: string): string {
		switch (role) {
			case 'system':
				return text.roleSystem;
			case 'assistant':
				return text.roleAssistant;
			case 'tool':
				return text.roleTool;
			default:
				return text.roleUser;
		}
	}

	function isFoldedByDefault(message: ExchangeMessage): boolean {
		return message.role === 'system' || message.role === 'tool';
	}
</script>

{#snippet messageBody(message: ExchangeMessage)}
	{#if message.reasoning}
		<p class="text-xs text-muted-foreground">{text.reasoningLabel}</p>
		<pre class="max-h-48 overflow-auto font-sans text-xs leading-relaxed whitespace-pre-wrap text-muted-foreground">{message.reasoning}</pre>
	{/if}
	{#if message.text}
		<pre class="max-h-96 overflow-auto font-sans text-sm leading-relaxed whitespace-pre-wrap">{message.text}</pre>
	{/if}
	{#if message.imageCount > 0}
		<Badge variant="outline" class="w-fit">{text.imagesAttached.replace('{count}', String(message.imageCount))}</Badge>
	{/if}
	{#each message.toolCalls as toolCall, index (`${toolCall.name}-${index}`)}
		<div class="flex flex-col gap-1 rounded-md bg-muted/50 px-2 py-1.5">
			<code class="text-xs font-medium">{toolCall.name}</code>
			<pre class="max-h-64 overflow-auto text-xs leading-relaxed whitespace-pre-wrap">{toolCall.arguments}</pre>
		</div>
	{/each}
{/snippet}

{#snippet bytesSection(label: string, bytes: string)}
	<Collapsible.Root>
		<div class="flex items-center gap-2">
			<Collapsible.Trigger class={buttonVariants({ variant: 'ghost', size: 'xs' })}>{label}</Collapsible.Trigger>
			<CopyButton text={bytes} variant="ghost" size="xs">
				<span>{text.copyBytes}</span>
			</CopyButton>
		</div>
		<Collapsible.Content>
			<pre class="mt-2 max-h-96 overflow-auto rounded-lg border bg-muted/30 px-3 py-2 text-xs leading-relaxed whitespace-pre-wrap">{bytes}</pre>
		</Collapsible.Content>
	</Collapsible.Root>
{/snippet}

<div class="flex flex-col gap-4">
	<div class="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
		{#if exchange.schemaName}
			<code class="rounded bg-muted px-1.5 py-0.5">{exchange.schemaName}</code>
		{/if}
		{#if exchange.seed !== undefined}
			<span>{text.seedLabel} <span class="font-mono text-foreground">{exchange.seed}</span></span>
		{/if}
		{#if exchange.servedBy}
			<span>{text.servedBy} <span class="text-foreground">{exchange.servedBy}</span></span>
		{/if}
		{#if exchange.toolNames.length > 0}
			<Collapsible.Root>
				<Collapsible.Trigger class={buttonVariants({ variant: 'ghost', size: 'xs' })}>
					{text.toolsOffered.replace('{count}', String(exchange.toolNames.length))}
				</Collapsible.Trigger>
				<Collapsible.Content class="mt-1 flex flex-wrap gap-1">
					{#each exchange.toolNames as toolName (toolName)}
						<code class="rounded bg-muted px-1.5 py-0.5">{toolName}</code>
					{/each}
				</Collapsible.Content>
			</Collapsible.Root>
		{/if}
	</div>

	<section class="flex flex-col gap-2">
		<h4 class="text-xs font-medium text-muted-foreground">{text.modelSaw}</h4>
		{#each exchange.messages as message, index (index)}
			<Collapsible.Root open={!isFoldedByDefault(message)} class="rounded-lg border">
				<Collapsible.Trigger class="group flex w-full items-center gap-2 px-3 py-2 text-left text-xs">
					<ChevronRightIcon class="size-3.5 shrink-0 transition-transform group-data-[state=open]:rotate-90" />
					<span class="font-medium">{roleLabel(message.role)}</span>
					<span class="min-w-0 flex-1 truncate text-muted-foreground">{message.text.slice(0, 120)}</span>
				</Collapsible.Trigger>
				<Collapsible.Content class="flex flex-col gap-2 border-t px-3 py-2">
					{@render messageBody(message)}
				</Collapsible.Content>
			</Collapsible.Root>
		{/each}
		{#if stateDocument}
			<Collapsible.Root class="rounded-lg border">
				<Collapsible.Trigger class="group flex w-full items-center gap-2 px-3 py-2 text-left text-xs">
					<ChevronRightIcon class="size-3.5 shrink-0 transition-transform group-data-[state=open]:rotate-90" />
					<span class="font-medium">{text.decisionState}</span>
					<span class="min-w-0 flex-1 truncate text-muted-foreground">{exchange.decisionQuestions.join(', ')}</span>
				</Collapsible.Trigger>
				<Collapsible.Content class="border-t px-3 py-2">
					<pre class="max-h-96 overflow-auto text-xs leading-relaxed whitespace-pre-wrap">{stateDocument}</pre>
				</Collapsible.Content>
			</Collapsible.Root>
		{/if}
	</section>

	{#if exchange.answer}
		<section class="flex flex-col gap-2">
			<h4 class="text-xs font-medium text-muted-foreground">{text.modelAnswered}</h4>
			<div class="flex flex-col gap-2 rounded-lg border bg-muted/20 px-3 py-2">
				{@render messageBody(exchange.answer)}
			</div>
		</section>
	{/if}

	{#if inputDocument}
		{@render bytesSection(text.decisionInput, inputDocument)}
	{/if}
	{#if exchange.request}
		{@render bytesSection(text.requestAsSent, exchange.request)}
	{/if}
	{#if exchange.response}
		{@render bytesSection(text.responseAsReceived, exchange.response)}
	{/if}
</div>
