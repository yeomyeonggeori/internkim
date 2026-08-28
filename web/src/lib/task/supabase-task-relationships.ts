import { supabase } from '$lib/supabase';

export async function updateSupabaseTaskParent(taskID: string, parentTaskID: string | null): Promise<void> {
	const result = await supabase()
		.rpc('task_parent_set', { target_task_id: taskID, target_parent_task_id: parentTaskID });
	if (result.error) {
		throw new Error(`Could not update the parent relationship for task ${taskID}: ${result.error.message}`);
	}
}

export async function updateSupabaseTaskParents(taskIDs: string[], parentTaskID: string): Promise<void> {
	if (taskIDs.length === 0) return;
	const result = await supabase()
		.rpc('task_children_link', { parent_id: parentTaskID, child_ids: taskIDs });
	if (result.error) {
		throw new Error(`Could not update parent relationships for ${taskIDs.length} tasks: ${result.error.message}`);
	}
}
