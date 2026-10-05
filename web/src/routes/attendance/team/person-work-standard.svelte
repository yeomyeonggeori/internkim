<script lang="ts">
 import { onDestroy } from 'svelte';
 import { Button } from '$lib/components/ui/button';
 import { createPageText } from '$lib/i18n/page-text.svelte';
 import { attendanceText } from '../text';
 import { supabaseWorkStatusInputs, attendanceWorkStatusFrom } from '$lib/attendance/supabase-work-status';
 import { WorkStatusState, setWorkStatusState } from '../work-status/work-status-state.svelte';
 import WorkStandardSummary from '../personal/work-standard-summary.svelte';
 import type { AttendanceSummary } from '../attendance-context.svelte';
 let {summary}: {summary: AttendanceSummary} = $props();
 const text = createPageText(attendanceText);
 const standardState = new WorkStatusState();
 setWorkStatusState(standardState);
 let open = $state(false);
 let generation = 0;
 onDestroy(() => {++generation; standardState.dispose();});
 async function toggle() {
  open = !open;
  if (!open || standardState.payload) return;
  const member = summary.members?.[0];
  if (!member?.memberID) return;
  const request = ++generation;
  standardState.isLoading = true;
  try {
   const asked = {period:'month' as const, anchor:summary.month+'-01'};
   const rows = await supabaseWorkStatusInputs([asked], undefined, {personID:member.memberID,name:member.displayName,email:member.email,isAdmin:summary.isAdmin});
   if(request!==generation) return;
   standardState.payload = attendanceWorkStatusFrom(rows, asked);
  } catch(error) {if(request===generation) standardState.errorMessage=String(error);}
  finally {if(request===generation) standardState.isLoading=false;}
 }
</script>
<Button variant="ghost" size="sm" aria-expanded={open} onclick={toggle}>{text.workStatus.standard}</Button>
{#if open}<WorkStandardSummary />{/if}
