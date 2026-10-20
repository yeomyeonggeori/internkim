import { z } from 'zod';

const carriedWorkspaceFileSchema = z.object({
	workspacePath: z.string().min(1),
	filename: z.string().optional(),
	contentBase64: z.string(),
	sha256: z.string().optional()
});

const carriedTransportSchema = z.object({
	transport: z.object({ workspaceFiles: z.array(carriedWorkspaceFileSchema).optional() }).optional()
});

export type CarriedWorkspaceFile = z.infer<typeof carriedWorkspaceFileSchema>;

export function workspaceFilesCarriedBy(payload: Record<string, unknown>): CarriedWorkspaceFile[] {
	const carried = carriedTransportSchema.safeParse(payload);
	if (!carried.success) throw new Error(`the workspace files this call carried are not readable: ${carried.error.message}`);
	return carried.data.transport?.workspaceFiles ?? [];
}
