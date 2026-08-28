<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import { sizeBadgeClass } from './task-style';
	import type { Task } from './task-types';

	type Props = {
		task: Task;
	};

	let { task }: Props = $props();
</script>

<Badge class={sizeBadgeClass(task.size)}>{task.size}</Badge>
