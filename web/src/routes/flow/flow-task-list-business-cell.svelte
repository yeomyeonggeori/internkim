<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import { flowBusinessBadgeStyle } from './flow-business-color';

	type Props = {
		label: string;
		color: string;
	};

	let { label, color }: Props = $props();
</script>

<Badge class="h-5 rounded-md border-transparent px-1.5 py-0 text-[11px] font-medium shadow-none" style={flowBusinessBadgeStyle(color)}>
	{label}
</Badge>
