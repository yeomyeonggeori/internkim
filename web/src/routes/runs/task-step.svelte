<script lang="ts">
	import * as Accordion from '$lib/components/ui/accordion';
	import CheckIcon from '@lucide/svelte/icons/check';
	import LoaderIcon from '@lucide/svelte/icons/loader';
	import MessageSquareIcon from '@lucide/svelte/icons/message-square';
	import XIcon from '@lucide/svelte/icons/x';
	import LLMCallDetail from './llm-call-detail.svelte';
	import * as Collapsible from '$lib/components/ui/collapsible';
	import { buttonVariants } from '$lib/components/ui/button';
	import ChevronRightIcon from '@lucide/svelte/icons/chevron-right';
	import RawDocument from './raw-document.svelte';
	import { formatDuration } from './runs-view';
	import StepSection from './step-section.svelte';
	import { stepDurationMS, type TaskStep } from './task-story';
	import type { TasksText } from './text';
	import TurnInputDetail from './turn-input-detail.svelte';

	let { step, isOpen, text }: { step: TaskStep; isOpen: boolean; text: TasksText } = $props();

	const StatusIcon = $derived(statusIcon());
	const duration = $derived(formatDuration(stepDurationMS(step)));
	const hasToolCall = $derived(step.input !== undefined || step.output !== undefined || (step.toolName !== undefined && step.failureText !== undefined));

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
	<div class="flex min-w-0 flex-col gap-1">
		<span class="text-xs text-muted-foreground">{label}</span>
		<RawDocument {document} />
	</div>
{/snippet}

<Accordion.Item value={step.key}>
	<Accordion.Trigger class="items-center gap-3">
		<StatusIcon aria-hidden="true" class="size-4 shrink-0 {step.isFailed ? 'text-destructive' : 'text-muted-foreground'}" />
		<span class="min-w-0 flex-1 font-normal {step.kind === 'reply' ? 'line-clamp-3 whitespace-pre-wrap' : 'truncate'}">{step.title}</span>
		{#if step.toolName}
			<code class="hidden shrink-0 text-xs font-normal text-muted-foreground sm:inline">{step.toolName}</code>
		{/if}
		<span class="w-12 shrink-0 text-right text-xs font-normal text-muted-foreground tabular-nums">{duration}</span>
	</Accordion.Trigger>
	<Accordion.Content class="flex min-w-0 flex-col gap-3 pb-4 pl-7">
		{#if isOpen}
			{#if hasToolCall}
				<StepSection title={text.stepToolCall} identifier={step.toolName}>
					<div class="grid min-w-0 gap-3 md:grid-cols-2">
						{#if step.input !== undefined}
							{@render labeled(text.stepInput, documentOf(step.input))}
						{/if}
						{#if step.failureText}
							<div class="flex min-w-0 flex-col gap-1">
								<span class="text-xs text-muted-foreground">{text.stepOutput}</span>
								<p class="text-sm text-destructive">{step.failureText}</p>
							</div>
						{:else if step.output !== undefined}
							{@render labeled(text.stepOutput, documentOf(step.output))}
						{/if}
					</div>
				</StepSection>
			{:else if step.failureText}
				<p class="text-sm text-destructive">{step.failureText}</p>
			{/if}
			{#each step.modelCalls as modelCall (modelCall.event.id ?? modelCall.event.createdAt)}
				<StepSection title={text.stepModelCall} identifier={modelCall.record.model}>
					<LLMCallDetail llmCallID={modelCall.event.id} record={modelCall.record} {text} isModelInHeader />
				</StepSection>
			{/each}
			{#if step.turnInput?.id}
				<Collapsible.Root class="flex flex-col gap-2">
					<Collapsible.Trigger class={buttonVariants({ variant: 'ghost', size: 'xs', class: 'group w-fit text-muted-foreground' })}>
						<ChevronRightIcon class="transition-transform group-data-[state=open]:rotate-90" />
						{text.turnInputLabel}
					</Collapsible.Trigger>
					<Collapsible.Content>
						<TurnInputDetail taskEventID={step.turnInput.id} {text} />
					</Collapsible.Content>
				</Collapsible.Root>
			{/if}
		{/if}
	</Accordion.Content>
</Accordion.Item>
