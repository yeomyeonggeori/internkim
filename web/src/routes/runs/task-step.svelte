<script lang="ts">
	import * as Accordion from '$lib/components/ui/accordion';
	import CheckIcon from '@lucide/svelte/icons/check';
	import LoaderIcon from '@lucide/svelte/icons/loader';
	import MessageSquareIcon from '@lucide/svelte/icons/message-square';
	import XIcon from '@lucide/svelte/icons/x';
	import LLMCallDetail from './llm-call-detail.svelte';
	import RawDocument from './raw-document.svelte';
	import { formatDuration } from './runs-view';
	import { stepDurationMS, type TaskStep } from './task-story';
	import type { TasksText } from './text';
	import TurnInputDetail from './turn-input-detail.svelte';

	let { step, isOpen, text }: { step: TaskStep; isOpen: boolean; text: TasksText } = $props();

	const StatusIcon = $derived(statusIcon());
	const duration = $derived(formatDuration(stepDurationMS(step)));

	function statusIcon() {
		if (step.isFailed) return XIcon;
		if (step.kind === 'reply') return MessageSquareIcon;
		if (!step.isFinished) return LoaderIcon;
		return CheckIcon;
	}

	function documentOf(value: unknown): string {
		return typeof value === 'string' ? value : JSON.stringify(value, undefined, 2);
	}
</script>

{#snippet labeled(label: string, document: string)}
	<div class="flex flex-col gap-1">
		<span class="text-xs text-muted-foreground">{label}</span>
		<RawDocument {document} />
	</div>
{/snippet}

<Accordion.Item value={step.key}>
	<Accordion.Trigger class="items-center gap-3">
		<StatusIcon aria-hidden="true" class="size-4 shrink-0 {step.isFailed ? 'text-destructive' : 'text-muted-foreground'}" />
		<span class="min-w-0 flex-1 truncate font-normal">{step.title}</span>
		{#if step.toolName}
			<code class="hidden shrink-0 text-xs font-normal text-muted-foreground sm:inline">{step.toolName}</code>
		{/if}
		<span class="w-12 shrink-0 text-right text-xs font-normal text-muted-foreground tabular-nums">{duration}</span>
	</Accordion.Trigger>
	<Accordion.Content class="flex flex-col gap-4 pl-7">
		{#if isOpen}
			{#if step.failureText}
				<p class="text-sm text-destructive">{step.failureText}</p>
			{/if}
			{#if step.kind === 'reply'}
				<p class="text-sm whitespace-pre-wrap">{step.title}</p>
			{/if}
			{#if step.input !== undefined}
				{@render labeled(text.stepInput, documentOf(step.input))}
			{/if}
			{#if step.output !== undefined}
				{@render labeled(text.stepOutput, documentOf(step.output))}
			{/if}
			{#each step.modelCalls as modelCall (modelCall.event.id ?? modelCall.event.createdAt)}
				<div class="flex flex-col gap-2">
					<span class="text-xs text-muted-foreground">{text.stepModelCall}</span>
					<LLMCallDetail llmCallID={modelCall.event.id} record={modelCall.record} {text} />
				</div>
			{/each}
			{#if step.turnInput?.id}
				<div class="flex flex-col gap-1">
					<span class="text-xs text-muted-foreground">{text.turnInputLabel}</span>
					<TurnInputDetail taskEventID={step.turnInput.id} {text} />
				</div>
			{/if}
		{/if}
	</Accordion.Content>
</Accordion.Item>
