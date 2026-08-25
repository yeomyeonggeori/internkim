<script lang="ts">
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import type { organizationDirectoryText } from './text';

	let {
		isOpen = false,
		text,
		onKeepEditing,
		onDiscard
	}: {
		isOpen?: boolean;
		text: typeof organizationDirectoryText.ko;
		onKeepEditing: () => void;
		onDiscard: () => void;
	} = $props();
</script>

<AlertDialog.Root open={isOpen} onOpenChange={(nextOpen) => !nextOpen && onKeepEditing()}>
	<AlertDialog.Content data-testid="organization-discard-edits-dialog">
		<AlertDialog.Header>
			<AlertDialog.Title>{text.discardEditsTitle}</AlertDialog.Title>
			<AlertDialog.Description>{text.discardEditsDescription}</AlertDialog.Description>
		</AlertDialog.Header>
		<AlertDialog.Footer>
			<AlertDialog.Cancel type="button" onclick={onKeepEditing}>{text.keepEditing}</AlertDialog.Cancel>
			<AlertDialog.Action type="button" variant="destructive" onclick={onDiscard}>{text.discardEdits}</AlertDialog.Action>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>
