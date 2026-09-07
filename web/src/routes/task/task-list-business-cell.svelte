<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import { taskDefinitionBadgeStyle } from './task-definition-colors';

	type Props = {
		label: string;
		color: string;
	};

	let { label, color }: Props = $props();
</script>

<Badge
	class="h-5 rounded-md border px-1.5 py-0 text-[11px] font-medium shadow-none"
	style={taskDefinitionBadgeStyle(color)}
>
	{label}
</Badge>
