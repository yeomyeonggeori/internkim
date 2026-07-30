export type LeaveEvidenceViolation = 'tooLarge' | 'tooMany' | 'invalidType';

export const leaveEvidenceMaximumFiles = 5;
export const leaveEvidenceMaximumFileSizeBytes = 10_000_000;
export const leaveEvidenceAcceptedFileTypes =
	'.jpg,.jpeg,.png,.pdf,image/jpeg,image/png,application/pdf';

export function validateLeaveEvidenceFiles(
	currentFileCount: number,
	files: File[]
): { acceptedFiles: File[]; violation?: LeaveEvidenceViolation } {
	const acceptedFiles: File[] = [];
	let violation: LeaveEvidenceViolation | undefined;
	for (const file of files) {
		const fileViolation = leaveEvidenceFileViolation(
			file,
			currentFileCount + acceptedFiles.length
		);
		if (fileViolation) {
			violation ??= fileViolation;
			continue;
		}
		acceptedFiles.push(file);
	}
	return { acceptedFiles, ...(violation ? { violation } : {}) };
}

function leaveEvidenceFileViolation(
	file: File,
	currentFileCount: number
): LeaveEvidenceViolation | undefined {
	if (file.size > leaveEvidenceMaximumFileSizeBytes) return 'tooLarge';
	if (currentFileCount >= leaveEvidenceMaximumFiles) return 'tooMany';
	const normalizedName = file.name.toLowerCase();
	const normalizedType = file.type.toLowerCase();
	if (
		normalizedName.endsWith('.jpg') ||
		normalizedName.endsWith('.jpeg') ||
		normalizedName.endsWith('.png') ||
		normalizedName.endsWith('.pdf') ||
		normalizedType === 'image/jpeg' ||
		normalizedType === 'image/png' ||
		normalizedType === 'application/pdf'
	) {
		return undefined;
	}
	return 'invalidType';
}
