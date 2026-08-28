<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import { taskDefinitionBadgeStyle, taskDefinitionOutlineBadgeStyle } from './task-definition-colors';

	type Props = {
		label: string;
		color: string;
		isOutlined?: boolean;
	};

	let { label, color, isOutlined = false }: Props = $props();
</script>

<Badge
	class="h-5 rounded-md border px-1.5 py-0 text-[11px] font-medium shadow-none"
	style={isOutlined ? taskDefinitionOutlineBadgeStyle(color) : taskDefinitionBadgeStyle(color)}
>
	{label}
</Badge>
