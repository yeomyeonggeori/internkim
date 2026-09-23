<script lang="ts">
	import LLMCallDetail from './llm-call-detail.svelte';
	import type { LLMCallRecord } from './llm-calls';
	import { formatCostUSD } from './runs-api';
	import { formatLatency } from './runs-view';
	import type { TasksText } from './text';
	import TimelineEvent from './timeline-event.svelte';

	let {
		value,
		isOpen,
		llmCallID,
		record,
		elapsed,
		text
	}: { value: string; isOpen: boolean; llmCallID?: string; record: LLMCallRecord; elapsed: string; text: TasksText } = $props();

	const meta = $derived([formatLatency(record.latencyMs), record.costUSD > 0 ? formatCostUSD(record.costUSD) : ''].filter(Boolean).join(' · '));
</script>

<TimelineEvent
	{value}
	lane={record.isError ? 'failure' : 'llm'}
	laneLabel={record.isError ? text.laneFailure : text.laneLLM}
	title={record.schemaName || record.kind}
	{meta}
	{elapsed}
	{isOpen}
>
	<LLMCallDetail {llmCallID} {record} {text} />
</TimelineEvent>
