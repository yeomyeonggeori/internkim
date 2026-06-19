<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import { sizeBadgeClass, statusBadgeClass } from './flow-style';
	import { flowBusinessLabel } from './flow-task-workspace-model';
	import type { FlowTaskEditorText } from './flow-task-editor-types';
	import type { FlowTask } from './flow-types';

	type Props = {
		taskDraft: FlowTask;
		text: FlowTaskEditorText;
		statusLabel: (status: string) => string;
	};

	let { taskDraft, text, statusLabel }: Props = $props();
</script>

<div class="flex flex-wrap gap-2">
	<Badge class={statusBadgeClass(taskDraft.status)}>{statusLabel(taskDraft.status)}</Badge>
	<Badge class={sizeBadgeClass(taskDraft.size)}>{taskDraft.size}</Badge>
	<Badge variant="outline">{flowBusinessLabel(taskDraft.business, text.businessFallback)}</Badge>
	<Badge variant="outline">{taskDraft.type}</Badge>
</div>
