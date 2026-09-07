<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import DefinitionBadge from '$lib/components/definition-badge.svelte';
	import { taskDefinitionBadgeStyle } from './task-definition-colors';
	import TaskStatusBadge from '$lib/components/task-status-badge.svelte';
	import { sizeBadgeClass } from './task-style';
	import { taskDefinitionLabel } from './task-workspace-model';
	import type { TaskEditorText } from './task-editor-types';
	import type { Task } from './task-types';

	type Props = {
		taskDraft: Task;
		text: TaskEditorText;
		statusLabel: (status: string) => string;
		businessColor: (business: string | null) => string;
		taskTypeColor: (type: string | null) => string;
	};

	let { taskDraft, text, statusLabel, businessColor, taskTypeColor }: Props = $props();
</script>

<div class="flex flex-wrap gap-2">
	<TaskStatusBadge status={taskDraft.status} label={statusLabel(taskDraft.status)} />
	<Badge class={sizeBadgeClass(taskDraft.size)}>{taskDraft.size}</Badge>
	<Badge class="border-transparent" style={taskDefinitionBadgeStyle(businessColor(taskDraft.business))}>
		{taskDefinitionLabel(taskDraft.business, text.etcLabel)}
	</Badge>
	<DefinitionBadge label={taskDefinitionLabel(taskDraft.type, text.etcLabel)} color={taskTypeColor(taskDraft.type)} />
</div>
