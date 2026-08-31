<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import { taskDefinitionBadgeStyle, taskDefinitionOutlineBadgeStyle } from './task-definition-colors';
	import { sizeBadgeClass, statusBadgeClass } from './task-style';
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
	<Badge class={statusBadgeClass(taskDraft.status)}>{statusLabel(taskDraft.status)}</Badge>
	<Badge class={sizeBadgeClass(taskDraft.size)}>{taskDraft.size}</Badge>
	<Badge class="border-transparent" style={taskDefinitionBadgeStyle(businessColor(taskDraft.business))}>
		{taskDefinitionLabel(taskDraft.business, text.etcLabel)}
	</Badge>
	<Badge variant="outline" style={taskDefinitionOutlineBadgeStyle(taskTypeColor(taskDraft.type))}>{taskDefinitionLabel(taskDraft.type, text.etcLabel)}</Badge>
</div>
