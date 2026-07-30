<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import { flowDefinitionBadgeStyle, flowDefinitionOutlineBadgeStyle } from './flow-definition-colors';
	import { sizeBadgeClass, statusBadgeClass } from './flow-style';
	import { flowBusinessLabel } from './flow-task-workspace-model';
	import type { FlowTaskEditorText } from './flow-task-editor-types';
	import type { FlowTask } from './flow-types';

	type Props = {
		taskDraft: FlowTask;
		text: FlowTaskEditorText;
		statusLabel: (status: string) => string;
		businessColor: (business: string) => string;
		taskTypeColor: (type: string) => string;
	};

	let { taskDraft, text, statusLabel, businessColor, taskTypeColor }: Props = $props();
</script>

<div class="flex flex-wrap gap-2">
	<Badge class={statusBadgeClass(taskDraft.status)}>{statusLabel(taskDraft.status)}</Badge>
	<Badge class={sizeBadgeClass(taskDraft.size)}>{taskDraft.size}</Badge>
	<Badge class="border-transparent" style={flowDefinitionBadgeStyle(businessColor(taskDraft.business))}>
		{flowBusinessLabel(taskDraft.business, text.businessFallback)}
	</Badge>
	<Badge variant="outline" style={flowDefinitionOutlineBadgeStyle(taskTypeColor(taskDraft.type))}>{taskDraft.type}</Badge>
</div>
