<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import { taskStatusIcon, taskStatusVariant } from '../../routes/task/task-status-style';

	type Props = {
		status: string;
		label: string;
	};

	let { status, label }: Props = $props();

	const StatusIcon = $derived(taskStatusIcon(status));
</script>

<Badge variant={taskStatusVariant(status)}>
	<StatusIcon data-icon="inline-start" aria-hidden="true" />
	{label}
</Badge>
