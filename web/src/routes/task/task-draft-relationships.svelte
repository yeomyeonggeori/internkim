<script lang="ts">
	import TaskRelationshipsSection from './task-relationships-section.svelte';
	import type { TaskRelationshipsText } from './task-editor-types';
	import type { Task } from './task-types';

	type Props = {
		taskDraft: Task;
		childTaskIDs: string[];
		tasks: Task[];
		currentMemberID: string;
		editable: boolean;
		text: TaskRelationshipsText;
		dateRangeLabel: string;
		taskTypeColor: (type: string | null) => string;
		statusLabel: (status: string) => string;
		onOpenTask: (task: Task) => void;
		onSetParent: (parentTaskID: string | undefined) => void;
		onSetChildren: (childTaskIDs: string[]) => void;
	};

	let {
		taskDraft,
		childTaskIDs,
		tasks,
		currentMemberID,
		editable,
		text,
		dateRangeLabel,
		taskTypeColor,
		statusLabel,
		onOpenTask,
		onSetParent,
		onSetChildren
	}: Props = $props();

	const draftTaskID = 'task-draft';

	let draft = $derived({ ...taskDraft, id: draftTaskID });
	let tasksWithDraft = $derived([
		draft,
		...tasks.map((task) => (childTaskIDs.includes(task.id) ? { ...task, parentTaskID: draftTaskID } : task))
	]);

	function setParent(taskID: string, parentTaskID?: string): boolean {
		if (taskID === draftTaskID) onSetParent(parentTaskID);
		else onSetChildren(childTaskIDs.filter((childTaskID) => childTaskID !== taskID));
		return true;
	}

	function setParents(taskIDs: string[]): boolean {
		onSetChildren([...childTaskIDs, ...taskIDs.filter((taskID) => !childTaskIDs.includes(taskID))]);
		return true;
	}
</script>

<TaskRelationshipsSection
	task={draft}
	tasks={tasksWithDraft}
	{currentMemberID}
	{editable}
	{text}
	{dateRangeLabel}
	{taskTypeColor}
	{statusLabel}
	{onOpenTask}
	onSetParent={setParent}
	onSetParents={setParents}
	onCreateChild={() => {}}
	allowTaskSwitching={false}
	canCreateChild={false}
/>
