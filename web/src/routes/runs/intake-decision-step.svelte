<script lang="ts">
	import * as Accordion from '$lib/components/ui/accordion';
	import InboxIcon from '@lucide/svelte/icons/inbox';
	import LLMCallDetail from './llm-call-detail.svelte';
	import StepSection from './step-section.svelte';
	import { formatDuration } from './runs-view';
	import type { ModelCall } from './task-story';
	import type { TasksText } from './text';

	let { decision, isOpen, text }: { decision: ModelCall; isOpen: boolean; text: TasksText } = $props();
</script>

<Accordion.Item value="intake-decision">
	<Accordion.Trigger class="items-center gap-3">
		<InboxIcon aria-hidden="true" class="size-4 shrink-0 text-muted-foreground" />
		<span class="min-w-0 flex-1 truncate font-normal">{text.intakeDecisionTitle}</span>
		<span class="w-12 shrink-0 text-right text-xs font-normal text-muted-foreground tabular-nums">{formatDuration(decision.record.latencyMs)}</span>
	</Accordion.Trigger>
	<Accordion.Content class="flex min-w-0 flex-col gap-3 pb-4 pl-7">
		{#if isOpen}
			<StepSection title={text.stepModelCall} identifier={decision.record.model}>
				<LLMCallDetail llmCallID={decision.event.id} record={decision.record} {text} isModelInHeader />
			</StepSection>
		{/if}
	</Accordion.Content>
</Accordion.Item>
