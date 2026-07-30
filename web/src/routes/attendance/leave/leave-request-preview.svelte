<script lang="ts">
	import { Spinner } from '$lib/components/ui/spinner';
	import type { AttendanceText } from '../text';
	import type { EmployeeLeavePreview } from './employee-leave-types';
	import { milliDaysValue } from './leave-history-model';

	type Props = {
		preview: EmployeeLeavePreview | null;
		isLoading: boolean;
		errorMessage: string;
		text: AttendanceText['leave'];
	};

	let { preview, isLoading, errorMessage, text }: Props = $props();

	function deduction(value: number): string {
		return `${milliDaysValue(value)}${text.dayUnit}`;
	}

	function excludedReason(reason: string): string {
		if (reason === 'holiday') return text.excludedHoliday;
		if (reason === 'nonWorkingDay') return text.excludedNonWorkingDay;
		return reason;
	}
</script>

<section class="space-y-2" aria-label={text.previewTitle} data-testid="leave-request-preview">
	<p class="text-sm font-medium">{text.previewTitle}</p>
	<div class="rounded-lg border bg-muted/20 p-4">
		{#if isLoading}
			<div class="flex min-h-20 items-center justify-center gap-2 text-sm text-muted-foreground">
				<Spinner class="size-4" />
				{text.previewLoading}
			</div>
		{:else if errorMessage}
			<p class="py-4 text-sm text-destructive">{errorMessage}</p>
		{:else if preview}
			<div class="grid gap-3">
				<div class="grid gap-2">
					{#each preview.occurrences as occurrence (`${occurrence.date}:${occurrence.startTime}`)}
						<div class="flex items-start justify-between gap-4 text-sm">
							<span class="text-muted-foreground">{occurrence.date}</span>
							<span class="text-right font-medium tabular-nums">
								{occurrence.startTime}–{occurrence.endTime}
								<span class="ml-2 text-muted-foreground">{deduction(occurrence.deductionMilliDays)}</span>
							</span>
						</div>
					{/each}
				</div>
				{#if preview.excludedDates.length}
					<div class="border-t pt-3">
						<p class="mb-2 text-xs font-medium text-muted-foreground">{text.excludedDates}</p>
						<div class="grid gap-1">
							{#each preview.excludedDates as excludedDate (excludedDate.date)}
								<div class="flex justify-between gap-3 text-xs text-muted-foreground">
									<span>{excludedDate.date}</span>
									<span>{excludedReason(excludedDate.reason)}</span>
								</div>
							{/each}
						</div>
					</div>
				{/if}
				<div class="flex items-center justify-between border-t pt-3 text-sm">
					<span class="font-medium">{text.totalDeduction}</span>
					<span class="font-semibold tabular-nums">{deduction(preview.totalDeductionMilliDays)}</span>
				</div>
			</div>
		{:else}
			<p class="py-4 text-center text-sm text-muted-foreground">{text.previewEmpty}</p>
		{/if}
	</div>
</section>
