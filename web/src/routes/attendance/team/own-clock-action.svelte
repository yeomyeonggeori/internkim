<script lang="ts">
 import { myAttendanceToday } from '$lib/attendance/my-attendance-today.svelte';
 import { Button } from '$lib/components/ui/button';
 import * as AlertDialog from '$lib/components/ui/alert-dialog';
 import * as Select from '$lib/components/ui/select';
 import LogInIcon from '@lucide/svelte/icons/log-in';
 import LogOutIcon from '@lucide/svelte/icons/log-out';
 import LoaderIcon from '@lucide/svelte/icons/loader-circle';
 import { createPageText } from '$lib/i18n/page-text.svelte';
 import { attendanceText } from '../text';
 const text = createPageText(attendanceText);
 let earlyOpen = $state(false);
 let priorOpen = $state(false);
 let selectedLocation = $state('');
 let closeTime = $state('');
 const fieldID = $props.id();
 const validLocation = $derived(myAttendanceToday.locations.some(location=>location.id===selectedLocation));
 $effect(() => {
  const locations = myAttendanceToday.locations;
  if (locations.some(location=>location.id===selectedLocation)) return;
  selectedLocation = locations.find(location=>location.isDefault)?.id ?? (locations.length===1?locations[0].id:'');
 });
 async function clock(confirmed = false) {
  try {await myAttendanceToday.clock(myAttendanceToday.nextKind, selectedLocation, confirmed);} catch {}
 }
 function continueClockIn() {
  if (myAttendanceToday.activeLeave) earlyOpen = true;
  else if (myAttendanceToday.clockInNobodyClosed) priorOpen = true;
  else void clock();
 }
 function beginClock() {
  if (myAttendanceToday.isSubmitting) return;
  if (myAttendanceToday.nextKind === 'clock_out') {void clock();return;}
  if (!validLocation) return;
  continueClockIn();
 }
 async function closeAndClockIn() {
  try {await myAttendanceToday.closeAndClockIn(closeTime, selectedLocation);priorOpen = false;closeTime = '';} catch {}
 }
</script>
<div class="flex w-full flex-col gap-3 sm:w-auto sm:items-end sm:gap-2">
 {#if myAttendanceToday.nextKind === 'clock_in' && myAttendanceToday.locations.length > 1}
  <Select.Root type="single" bind:value={selectedLocation} disabled={myAttendanceToday.isSubmitting}><Select.Trigger class="w-full sm:w-36" aria-label={text.location}>{myAttendanceToday.locations.find(location=>location.id===selectedLocation)?.name ?? text.location}</Select.Trigger><Select.Content><Select.Group>{#each myAttendanceToday.locations as location (location.id)}<Select.Item value={location.id} label={location.name}>{location.name}</Select.Item>{/each}</Select.Group></Select.Content></Select.Root>
 {/if}
 {#if myAttendanceToday.nextKind === 'clock_in' && myAttendanceToday.locations.length===0}<p role="alert" class="text-xs text-destructive">{text.noConfiguredWorkLocation}</p>{/if}
 <Button size="sm" class="w-full gap-3 sm:w-auto sm:gap-1" onclick={beginClock} disabled={!myAttendanceToday.summary || myAttendanceToday.isSubmitting || (myAttendanceToday.nextKind==='clock_in' && !validLocation)}>
  {#if myAttendanceToday.isSubmitting}<LoaderIcon class="size-4 animate-spin" />
  {:else if myAttendanceToday.nextKind === 'clock_out'}<LogOutIcon class="size-4 sm:hidden" />
  {:else}<LogInIcon class="size-4 sm:hidden" />{/if}
  {myAttendanceToday.nextKind === 'clock_out' ? text.clockOut : text.clockIn}
 </Button>
 {#if myAttendanceToday.clockFailure}<p role="alert" class="mt-2 max-w-sm text-xs text-destructive">{myAttendanceToday.clockFailure}</p>{/if}
</div>

<AlertDialog.Root bind:open={earlyOpen}><AlertDialog.Content><AlertDialog.Header><AlertDialog.Title>{text.earlyReturnConfirmTitle}</AlertDialog.Title><AlertDialog.Description>{text.earlyReturnConfirmDescriptionTemplate.replace('{endTime}',myAttendanceToday.activeLeave?.endTime ?? '')}</AlertDialog.Description></AlertDialog.Header><AlertDialog.Footer><AlertDialog.Cancel>{text.cancel}</AlertDialog.Cancel><AlertDialog.Action onclick={()=>void clock(true)} disabled={myAttendanceToday.isSubmitting}>{text.earlyReturnConfirmAction}</AlertDialog.Action></AlertDialog.Footer></AlertDialog.Content></AlertDialog.Root>
<AlertDialog.Root bind:open={priorOpen}><AlertDialog.Content><AlertDialog.Header><AlertDialog.Title>{text.clockOutNobodyRecordedTitle}</AlertDialog.Title><AlertDialog.Description>{text.clockOutNobodyRecordedDescriptionTemplate.replace('{date}',myAttendanceToday.clockInNobodyClosed?.localDate ?? '').replace('{time}',myAttendanceToday.clockInNobodyClosed?.localTime ?? '')}</AlertDialog.Description></AlertDialog.Header><label class="grid gap-2 text-sm" for={fieldID}>{text.clockOutNobodyRecordedLabel}<input id={fieldID} class="h-9 rounded-md border bg-background px-3" type="time" bind:value={closeTime} required /></label><AlertDialog.Footer><AlertDialog.Cancel>{text.cancel}</AlertDialog.Cancel><AlertDialog.Action onclick={closeAndClockIn} disabled={!closeTime || myAttendanceToday.isSubmitting}>{text.clockOutNobodyRecordedAction}</AlertDialog.Action></AlertDialog.Footer></AlertDialog.Content></AlertDialog.Root>
