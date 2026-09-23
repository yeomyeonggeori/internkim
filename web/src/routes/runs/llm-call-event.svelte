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
		createdAt,
		text
	}: { value: string; isOpen: boolean; llmCallID?: string; record: LLMCallRecord; createdAt?: string; text: TasksText } = $props();
</script>

<TimelineEvent
	{value}
	title={`llm.call · ${record.schemaName || record.kind}`}
	duration={formatLatency(record.latencyMs)}
	cost={record.costUSD > 0 ? formatCostUSD(record.costUSD) : ''}
	{createdAt}
	isFailed={record.isError}
	{isOpen}
>
	<LLMCallDetail {llmCallID} {record} {text} />
</TimelineEvent>
