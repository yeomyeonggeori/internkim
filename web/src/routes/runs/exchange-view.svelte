<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import { CopyButton } from '$lib/components/ui/copy-button';
	import FactList, { type Fact } from './fact-list.svelte';
	import type { Exchange, ExchangeMessage } from './llm-calls';
	import RawDocument from './raw-document.svelte';
	import type { TasksText } from './text';

	let { exchange, callFacts, text }: { exchange: Exchange; callFacts: Fact[]; text: TasksText } = $props();

	const stateDocument = $derived(exchange.decisionState === undefined ? '' : JSON.stringify(exchange.decisionState, undefined, 2));
	const facts = $derived([...callFacts, ...exchangeFacts()]);

	function exchangeFacts(): Fact[] {
		const facts: Fact[] = [];
		if (exchange.servedBy) facts.push({ label: text.servedBy, value: exchange.servedBy });
		if (exchange.seed !== undefined) facts.push({ label: text.seedLabel, value: String(exchange.seed) });
		if (exchange.toolNames.length > 0) facts.push({ label: text.toolsOffered, value: text.countValue.replace('{count}', String(exchange.toolNames.length)) });
		return facts;
	}
	const copyableDocuments = $derived(
		[
			{ label: text.requestAsSent, document: exchange.request },
			{ label: text.responseAsReceived, document: exchange.response },
			{ label: text.decisionInput, document: exchange.input === undefined ? '' : JSON.stringify(exchange.input, undefined, 2) }
		].filter((copyable) => copyable.document)
	);

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
</script>

{#snippet transcriptEntry(label: string, message: ExchangeMessage)}
	<div class="flex flex-col gap-1">
		<span class="text-xs text-muted-foreground">{label}</span>
		{#if message.reasoning}
			<pre class="max-h-32 overflow-auto font-sans text-xs leading-relaxed whitespace-pre-wrap text-muted-foreground">{message.reasoning}</pre>
		{/if}
		{#if message.text}
			<pre class="max-h-48 overflow-auto font-sans text-sm leading-relaxed whitespace-pre-wrap">{message.text}</pre>
		{/if}
		{#if message.imageCount > 0}
			<Badge variant="outline" class="w-fit">{text.imagesAttached.replace('{count}', String(message.imageCount))}</Badge>
		{/if}
		{#each message.toolCalls as toolCall, index (`${toolCall.name}-${index}`)}
			<pre class="max-h-48 overflow-auto rounded-md bg-muted/40 px-2 py-1.5 text-xs leading-relaxed whitespace-pre-wrap">{toolCall.name} {toolCall.arguments}</pre>
		{/each}
	</div>
{/snippet}

<div class="flex flex-col gap-4">
	<FactList {facts} />
	{#each exchange.messages as message, index (index)}
		{@render transcriptEntry(roleLabel(message.role), message)}
	{/each}
	{#if stateDocument}
		<div class="flex flex-col gap-1">
			<span class="text-xs text-muted-foreground">{text.decisionState}</span>
			<RawDocument document={stateDocument} />
		</div>
	{/if}
	{#if exchange.answer}
		{@render transcriptEntry(text.modelAnswered, exchange.answer)}
	{/if}
	{#if copyableDocuments.length > 0}
		<div class="flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-muted-foreground">
			{#each copyableDocuments as copyable (copyable.label)}
				<span class="flex items-center">
					{copyable.label}
					<CopyButton text={copyable.document} size="icon-xs" />
				</span>
			{/each}
		</div>
	{/if}
</div>
