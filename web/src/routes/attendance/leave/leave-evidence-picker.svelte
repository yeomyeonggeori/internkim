<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import * as FileDropZone from '$lib/components/ui/file-drop-zone';
	import UploadIcon from '@lucide/svelte/icons/upload';
	import XIcon from '@lucide/svelte/icons/x';
	import type { AttendanceText } from '../text';
	import type { EmployeeLeaveAttachment } from './employee-leave-types';
	import {
		leaveEvidenceAcceptedFileTypes,
		leaveEvidenceMaximumFiles,
		leaveEvidenceMaximumFileSizeBytes,
		validateLeaveEvidenceFiles,
		type LeaveEvidenceViolation
	} from './leave-evidence-policy';

	type Props = {
		files: File[];
		onFilesChange: (files: File[]) => void;
		existingAttachments?: EmployeeLeaveAttachment[];
		onExistingAttachmentRemove?: (attachmentID: string) => void;
		disabled?: boolean;
		text: AttendanceText['leave'];
	};

	let {
		files,
		onFilesChange,
		existingAttachments = [],
		onExistingAttachmentRemove,
		disabled = false,
		text
	}: Props = $props();
	let errorMessage = $state('');

	async function addFiles(uploadedFiles: File[]): Promise<void> {
		const result = validateLeaveEvidenceFiles(
			files.length + existingAttachments.length,
			uploadedFiles
		);
		errorMessage = result.violation ? violationMessage(result.violation) : '';
		onFilesChange([...files, ...result.acceptedFiles]);
	}

	function removeFile(fileIndex: number): void {
		onFilesChange(files.filter((_, index) => index !== fileIndex));
		errorMessage = '';
	}

	function rejectFile(reason: FileDropZone.FileRejectedReason): void {
		if (reason === 'Maximum file size exceeded') {
			errorMessage = violationMessage('tooLarge');
			return;
		}
		if (reason === 'Maximum files uploaded') {
			errorMessage = violationMessage('tooMany');
			return;
		}
		errorMessage = violationMessage('invalidType');
	}

	function violationMessage(violation: LeaveEvidenceViolation): string {
		if (violation === 'tooLarge') return text.evidenceTooLarge;
		if (violation === 'tooMany') return text.evidenceTooMany;
		return text.evidenceInvalidType;
	}

	function fileSize(sizeBytes: number): string {
		if (sizeBytes < 1_000_000) return `${Math.max(1, Math.round(sizeBytes / 1_000))} KB`;
		return `${(sizeBytes / 1_000_000).toFixed(1)} MB`;
	}
</script>

<div class="space-y-2" data-testid="leave-evidence-picker">
	<div>
		<p class="text-sm font-medium">{text.evidenceLabel}</p>
		<p class="text-xs text-muted-foreground">{text.evidenceDescription}</p>
	</div>

	<FileDropZone.Root
		onUpload={addFiles}
		onFileRejected={({ reason }) => rejectFile(reason)}
		maxFiles={leaveEvidenceMaximumFiles}
		fileCount={files.length + existingAttachments.length}
		maxFileSize={leaveEvidenceMaximumFileSizeBytes}
		accept={leaveEvidenceAcceptedFileTypes}
		{disabled}
	>
		<FileDropZone.Trigger>
			<div
				class="flex min-h-28 cursor-pointer flex-col items-center justify-center gap-2 rounded-lg border border-dashed px-4 py-5 text-center transition-colors hover:bg-muted/40 group-aria-disabled/file-drop-zone-trigger:pointer-events-none group-aria-disabled/file-drop-zone-trigger:opacity-50"
			>
				<UploadIcon class="size-6 text-muted-foreground" />
				<div>
					<p class="text-sm font-medium">{text.evidenceDrop}</p>
					<p class="mt-1 text-xs text-muted-foreground">{text.evidenceLimit}</p>
				</div>
			</div>
		</FileDropZone.Trigger>
	</FileDropZone.Root>

	{#if existingAttachments.length || files.length}
		<div class="grid gap-1.5">
			{#each existingAttachments as attachment (attachment.id)}
				<div class="flex min-w-0 items-center gap-2 rounded-md bg-muted/50 px-3 py-1.5 text-xs">
					<a href={attachment.downloadURL} class="min-w-0 flex-1 truncate hover:underline">
						{attachment.fileName}
					</a>
					<span class="shrink-0 text-muted-foreground">{fileSize(attachment.sizeBytes)}</span>
					{#if onExistingAttachmentRemove}
						<Button
							type="button"
							variant="ghost"
							size="icon-xs"
							aria-label={text.removeEvidence.replace('{name}', attachment.fileName)}
							onclick={() => onExistingAttachmentRemove?.(attachment.id)}
							{disabled}
						>
							<XIcon />
						</Button>
					{/if}
				</div>
			{/each}
			{#each files as file, index (`${file.name}:${file.size}:${index}`)}
				<div class="flex min-w-0 items-center gap-2 rounded-md bg-muted/50 px-3 py-1.5 text-xs">
					<span class="min-w-0 flex-1 truncate">{file.name}</span>
					<span class="shrink-0 text-muted-foreground">{fileSize(file.size)}</span>
					<Button
						type="button"
						variant="ghost"
						size="icon-xs"
						aria-label={text.removeEvidence.replace('{name}', file.name)}
						onclick={() => removeFile(index)}
						{disabled}
					>
						<XIcon />
					</Button>
				</div>
			{/each}
		</div>
	{/if}

	{#if errorMessage}
		<p class="text-xs text-destructive">{errorMessage}</p>
	{/if}
</div>
