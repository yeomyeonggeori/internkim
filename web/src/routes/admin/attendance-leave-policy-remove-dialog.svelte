<script lang="ts">
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import type { AdminPageText } from './admin-types';

	type Props = {
		open: boolean;
		leaveTypeName: string;
		isSaving: boolean;
		text: AdminPageText;
		onOpenChange: (open: boolean) => void;
		onConfirm: () => void;
	};

	let { open, leaveTypeName, isSaving, text, onOpenChange, onConfirm }: Props = $props();
</script>

<AlertDialog.Root {open} {onOpenChange}>
	<AlertDialog.Content>
		<AlertDialog.Header>
			<AlertDialog.Title>{text.attendanceSettings.removeConfirmationTitle}</AlertDialog.Title>
			<AlertDialog.Description>
				{text.attendanceSettings.removeConfirmationDescription.replace(
					'{name}',
					leaveTypeName
				)}
			</AlertDialog.Description>
		</AlertDialog.Header>
		<AlertDialog.Footer>
			<AlertDialog.Cancel disabled={isSaving}>
				{text.attendanceSettings.removeConfirmationCancel}
			</AlertDialog.Cancel>
			<AlertDialog.Action disabled={isSaving} onclick={onConfirm}>
				{text.attendanceSettings.removeConfirmationAction}
			</AlertDialog.Action>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>
