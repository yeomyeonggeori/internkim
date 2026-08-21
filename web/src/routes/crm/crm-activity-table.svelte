<script lang="ts">
	import { currentLocale } from '$lib/i18n/locale.svelte';
	import { Badge } from '$lib/components/ui/badge';
	import * as Table from '$lib/components/ui/table';
	import ListPaginationFooter from '$lib/components/list-pagination-footer.svelte';
	import type { CRMOrganization, CRMActivity } from './crm-types';
	import { crmLabel } from './crm-labels';
	import { findOrganizationByID, formatCRMDateTime } from './crm-view-model';
	import type { CRMText } from './text';

	type Props = {
		activities: CRMActivity[];
		organizations: CRMOrganization[];
		text: CRMText;
		onEdit: (activityID: string) => void;
	};

	let { activities, organizations, text, onEdit }: Props = $props();
	const pageSize = 10;
	let pageIndex = $state(0);
	let pageCount = $derived(Math.max(1, Math.ceil(activities.length / pageSize)));
	let visibleActivities = $derived(activities.slice(pageIndex * pageSize, (pageIndex + 1) * pageSize));

	function previousPage(): void {
		pageIndex = Math.max(0, pageIndex - 1);
	}

	function nextPage(): void {
		pageIndex = Math.min(pageCount - 1, pageIndex + 1);
	}

	function handleRowKeydown(event: KeyboardEvent, activityID: string): void {
		if (event.key !== 'Enter' && event.key !== ' ') return;
		event.preventDefault();
		onEdit(activityID);
	}

	$effect(() => {
		if (pageIndex >= pageCount) pageIndex = pageCount - 1;
		if (pageIndex < 0) pageIndex = 0;
	});
</script>

<div class="min-w-0 max-w-full overflow-hidden rounded-lg border bg-card shadow-sm">
	<div class="min-w-0">
		<Table.Root class="table-fixed">
			<Table.Header class="bg-muted/50 text-left">
				<Table.Row class="hover:bg-transparent">
					<Table.Head class="w-[35%] pl-4 sm:w-[25%] md:w-[20%] lg:w-[16%] xl:w-[13%]">{text.occurredAt}</Table.Head>
					<Table.Head class="hidden w-[25%] sm:table-cell md:w-[20%] lg:w-[16%] xl:w-[13%]">{text.organizationName}</Table.Head>
					<Table.Head class="hidden w-[8%] xl:table-cell">{text.business}</Table.Head>
					<Table.Head class="hidden w-[15%] text-center sm:table-cell lg:w-[12%] xl:w-[9%]">{text.activityKind}</Table.Head>
					<Table.Head class="hidden w-[15%] text-center md:table-cell lg:w-[12%] xl:w-[12%]">{text.activityStatus}</Table.Head>
					<Table.Head class="w-[65%] sm:w-[35%] md:w-[30%] lg:w-[24%] xl:w-[17%]">{text.activityTitle}</Table.Head>
					<Table.Head class="hidden w-[20%] lg:table-cell">{text.details}</Table.Head>
					<Table.Head class="hidden w-[8%] xl:table-cell">{text.linkedCalendar}</Table.Head>
				</Table.Row>
			</Table.Header>
			<Table.Body class="text-left">
				{#each visibleActivities as activity (activity.id)}
					{@const organization = findOrganizationByID(organizations, activity.organizationID)}
					<Table.Row
						class="cursor-pointer align-top hover:bg-muted/40 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-inset"
						tabindex={0}
						aria-label={`${text.editActivity} · ${activity.title}`}
						onclick={() => onEdit(activity.id)}
						onkeydown={(event) => handleRowKeydown(event, activity.id)}
					>
						<Table.Cell class="whitespace-normal pl-4 text-muted-foreground">{formatCRMDateTime(activity.occurredAt, currentLocale.value)}</Table.Cell>
						<Table.Cell class="hidden whitespace-normal font-medium sm:table-cell"><p class="truncate">{organization?.name ?? text.none}</p></Table.Cell>
						<Table.Cell class="hidden whitespace-normal xl:table-cell"><p class="truncate">{activity.business}</p></Table.Cell>
						<Table.Cell class="hidden whitespace-normal text-center sm:table-cell"><Badge variant="outline" data-crm-centered-pill>{crmLabel(text.activityKinds, activity.kind)}</Badge></Table.Cell>
						<Table.Cell class="hidden whitespace-normal text-center md:table-cell">
							<div class="flex justify-center">
								{#if activity.taskStatus}
									<Badge variant="secondary" data-crm-centered-pill>{crmLabel(text.nextActionStatuses, activity.taskStatus)}</Badge>
								{:else if activity.taskID}
									<Badge variant="outline" data-crm-centered-pill>{text.linkedTask}</Badge>
								{:else}
									<Badge variant="destructive" data-crm-centered-pill>{text.missingTask}</Badge>
								{/if}
							</div>
						</Table.Cell>
						<Table.Cell class="whitespace-normal font-medium"><span class="line-clamp-2">{activity.title}</span></Table.Cell>
						<Table.Cell class="hidden whitespace-normal text-sm text-muted-foreground lg:table-cell"><p class="line-clamp-2">{activity.summary}</p></Table.Cell>
						<Table.Cell class="hidden whitespace-normal xl:table-cell">
							<div class="flex justify-start">
								{#if activity.calendarEventID}
									<Badge variant="outline" data-crm-leading-pill>{text.calendarRegistered}</Badge>
								{:else if activity.calendarRegistrationState === 'failed'}
									<Badge variant="destructive" data-crm-leading-pill>{text.calendarCreateError}</Badge>
								{:else}
									<span class="text-sm text-muted-foreground">{text.none}</span>
								{/if}
							</div>
						</Table.Cell>
					</Table.Row>
				{:else}
					<Table.Row class="hover:bg-transparent"><Table.Cell colspan={8} class="py-10 text-center text-sm text-muted-foreground">{text.noActivities}</Table.Cell></Table.Row>
				{/each}
			</Table.Body>
		</Table.Root>
	</div>
	<div class="border-t bg-card px-3 py-3">
		<ListPaginationFooter
			totalItems={activities.length}
			{pageIndex}
			{pageSize}
			{pageCount}
			canPreviousPage={pageIndex > 0}
			canNextPage={pageIndex < pageCount - 1}
			{previousPage}
			{nextPage}
			summary={text.paginationSummary}
			previousLabel={text.paginationPrevious}
			nextLabel={text.paginationNext}
		/>
	</div>
</div>
