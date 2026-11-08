<script lang="ts">
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import type { ChildCompletionQuestion } from './task-board-controller.svelte';

	type Props = {
		question: ChildCompletionQuestion | null;
		text: { title: string; description: string; withChildren: string; alone: string };
	};

	let { question, text }: Props = $props();
</script>

<AlertDialog.Root open={question !== null} onOpenChange={(open) => { if (!open) question?.answer('cancelled'); }}>
	<AlertDialog.Content>
		<AlertDialog.Header>
			<AlertDialog.Title>{text.title.replace('{count}', String(question?.unfinishedChildren ?? 0))}</AlertDialog.Title>
			<AlertDialog.Description>{text.description.replace('{task}', question?.taskTitle ?? '')}</AlertDialog.Description>
		</AlertDialog.Header>
		<AlertDialog.Footer>
			<AlertDialog.Action variant="outline" onclick={() => question?.answer('alone')}>{text.alone}</AlertDialog.Action>
			<AlertDialog.Action onclick={() => question?.answer('with_children')}>{text.withChildren}</AlertDialog.Action>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>
