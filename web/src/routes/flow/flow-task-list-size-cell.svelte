<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import { flowDefinitionBadgeStyle } from './flow-definition-colors';
	import { sizeBadgeClass } from './flow-style';
	import type { FlowTask } from './flow-types';

	type Props = {
		task: FlowTask;
		color?: string;
	};

	let { task, color = '' }: Props = $props();
</script>

<Badge class={color ? 'rounded-md border-transparent font-mono tabular-nums shadow-none' : sizeBadgeClass(task.size)} style={flowDefinitionBadgeStyle(color)}>{task.size}</Badge>
