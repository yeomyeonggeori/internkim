<script lang="ts">
 import * as Select from '$lib/components/ui/select';
 import { currentLocale } from '$lib/i18n/locale.svelte';
 import { attendanceChangeReasons, attendanceChangeReasonLabels, isAttendanceChangeReason } from '$lib/attendance/change-reason';
 let {value = $bindable(''), disabled = false, label}: {value?: string; disabled?: boolean; label: string} = $props();
 const labels = $derived(attendanceChangeReasonLabels[currentLocale.value === 'ko' ? 'ko' : 'en']);
</script>
<label class="grid gap-1.5 text-sm font-medium"><span>{label}</span><Select.Root type="single" bind:value {disabled}><Select.Trigger class="w-full" aria-label={label}>{isAttendanceChangeReason(value) ? labels[value] : label}</Select.Trigger><Select.Content><Select.Group>{#each attendanceChangeReasons as code}<Select.Item value={code} label={labels[code]}>{labels[code]}</Select.Item>{/each}</Select.Group></Select.Content></Select.Root></label>
