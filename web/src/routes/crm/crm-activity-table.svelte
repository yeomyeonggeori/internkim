<script lang="ts">
	import { currentLocale } from '$lib/i18n/locale.svelte';
	import { Badge } from '$lib/components/ui/badge';
	import { cn } from '$lib/utils';
	import { sizeBadgeClass } from '../task/task-style';
	import { taskBusinessColor, taskTypeColor } from '../task/task-definition-colors';
	import type { TaskDefinitions } from '../task/task-types';
	import DefinitionBadge from '$lib/components/definition-badge.svelte';
	import TaskListBusinessCell from '../task/task-list-business-cell.svelte';
	import * as Table from '$lib/components/ui/table';
	import ListPaginationFooter from '$lib/components/list-pagination-footer.svelte';
	import PersonChipList from '$lib/components/person-chip-list.svelte';
	import CalendarCheckIcon from '@lucide/svelte/icons/calendar-check';
	import CalendarXIcon from '@lucide/svelte/icons/calendar-x';
	import type { UserRecord } from '$lib/organization/types';
	import type { CRMOrganization, CRMActivity, CRMOpportunity } from './crm-types';
	import { crmLabel } from './crm-labels';
	import CRMTableColumnHeader from './crm-table-column-header.svelte';
	import { nextSortState, sortRows, type CRMSortComparators, type CRMSortState } from './crm-table-sort';
	import { findOrganizationByID, formatCRMDate } from './crm-view-model';
	import TaskStatusBadge from '$lib/components/task-status-badge.svelte';
	import { taskStatusRank } from '../task/task-status-style';
	import { taskStatusLabelFrom } from '../task/task-status';
	import type { CRMText } from './text';

	type Props = {
		activities: CRMActivity[];
		organizations: CRMOrganization[];
		opportunities: CRMOpportunity[];
		people: UserRecord[];
		taskDefinitions: TaskDefinitions;
		text: CRMText;
		onEdit: (activityID: string) => void;
	};

	let { activities, organizations, opportunities, people, taskDefinitions, text, onEdit }: Props = $props();
	function emailOf(memberID: string): string {
		return people.find((person) => person.memberID === memberID)?.email ?? '';
	}

	const comparators: CRMSortComparators<CRMActivity> = {
		title: (activity) => activity.title,
		kind: (activity) => crmLabel(text.activityKinds, activity.kind),
		business: (activity) => activity.business,
		size: (activity) => activity.size,
		organization: (activity) => findOrganizationByID(organizations, activity.organizationID)?.name ?? '',
		endsAt: (activity) => activity.calendarEndsAt ?? '',
		occurredAt: (activity) => activity.occurredAt,
		owner: (activity) => activity.participantNames[0] ?? '',
		status: (activity) => taskStatusRank(activity.taskStatus)
	};

	const pageSize = 10;
	let pageIndex = $state(0);
	let sort = $state<CRMSortState | null>({ key: 'status', direction: 'descending' });
	let sortedActivities = $derived(
		sortRows(activities, sort, comparators, {
			read: (activity: CRMActivity) => activity.occurredAt,
			direction: 'descending'
		})
	);
	let pageCount = $derived(Math.max(1, Math.ceil(sortedActivities.length / pageSize)));
	let visibleActivities = $derived(sortedActivities.slice(pageIndex * pageSize, (pageIndex + 1) * pageSize));

	function toggleSort(key: string): void {
		sort = nextSortState(sort, key);
		pageIndex = 0;
	}

	function ariaSort(key: string): 'ascending' | 'descending' | 'none' {
		return sort?.key === key ? sort.direction : 'none';
	}

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
		<Table.Root class="table-auto">
			<Table.Header class="bg-muted/50 text-left">
				<Table.Row class="hover:bg-transparent">
					<Table.Head class="w-full pl-4" aria-sort={ariaSort('title')}><CRMTableColumnHeader label={text.activity} sortKey="title" {sort} onSort={toggleSort} /></Table.Head>
					<Table.Head class="hidden whitespace-nowrap sm:table-cell" aria-sort={ariaSort('kind')}><CRMTableColumnHeader label={text.columnKind} sortKey="kind" {sort} onSort={toggleSort} /></Table.Head>
					<Table.Head class="hidden whitespace-nowrap xl:table-cell" aria-sort={ariaSort('business')}><CRMTableColumnHeader label={text.business} sortKey="business" {sort} onSort={toggleSort} /></Table.Head>
					<Table.Head class="hidden whitespace-nowrap xl:table-cell" aria-sort={ariaSort('size')}><CRMTableColumnHeader label={text.columnSize} sortKey="size" {sort} onSort={toggleSort} /></Table.Head>
					<Table.Head class="hidden whitespace-nowrap sm:table-cell" aria-sort={ariaSort('organization')}><CRMTableColumnHeader label={text.organizationName} sortKey="organization" {sort} onSort={toggleSort} /></Table.Head>
					<Table.Head class="hidden whitespace-nowrap xl:table-cell">{text.opportunity}</Table.Head>
					<Table.Head class="hidden whitespace-nowrap text-right tabular-nums md:table-cell" aria-sort={ariaSort('occurredAt')}><CRMTableColumnHeader label={text.columnStartDate} sortKey="occurredAt" {sort} onSort={toggleSort} /></Table.Head>
					<Table.Head class="hidden whitespace-nowrap text-right tabular-nums xl:table-cell" aria-sort={ariaSort('endsAt')}><CRMTableColumnHeader label={text.columnEndDate} sortKey="endsAt" {sort} onSort={toggleSort} /></Table.Head>
					<Table.Head class="hidden whitespace-nowrap lg:table-cell" aria-sort={ariaSort('owner')}><CRMTableColumnHeader label={text.assignee} sortKey="owner" {sort} onSort={toggleSort} /></Table.Head>
					<Table.Head class="whitespace-nowrap pr-6" aria-sort={ariaSort('status')}><CRMTableColumnHeader label={text.status} sortKey="status" {sort} onSort={toggleSort} /></Table.Head>
				</Table.Row>
			</Table.Header>
			<Table.Body class="text-left">
				{#each visibleActivities as activity (activity.id)}
					{@const organization = findOrganizationByID(organizations, activity.organizationID)}
					{@const opportunity = opportunities.find((candidate) => candidate.id === activity.opportunityID)}
					<Table.Row
						class="cursor-pointer align-top hover:bg-muted/40 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-inset"
						tabindex={0}
						aria-label={`${text.editActivity} · ${activity.title}`}
						onclick={() => onEdit(activity.id)}
						onkeydown={(event) => handleRowKeydown(event, activity.id)}
					>
						<Table.Cell class="w-full whitespace-normal pl-4 font-medium">
							<div class="flex min-w-0 items-center gap-1.5">
								<span class="line-clamp-2">{activity.title}</span>
								{#if activity.calendarEventID}
									<CalendarCheckIcon class="size-3.5 shrink-0 text-muted-foreground" aria-label={text.calendarRegistered} />
								{:else if activity.calendarRegistrationState === 'failed'}
									<CalendarXIcon class="size-3.5 shrink-0 text-destructive" aria-label={text.calendarCreateError} />
								{/if}
							</div>
						</Table.Cell>
						<Table.Cell class="hidden whitespace-nowrap sm:table-cell"><DefinitionBadge label={crmLabel(text.activityKinds, activity.kind)} color={taskTypeColor(activity.kind, taskDefinitions)} /></Table.Cell>
						<Table.Cell class="hidden whitespace-nowrap xl:table-cell">
							{#if activity.business}
								<TaskListBusinessCell label={activity.business} color={taskBusinessColor(activity.business, taskDefinitions)} />
							{:else}
								<span class="text-muted-foreground">{text.none}</span>
							{/if}
						</Table.Cell>
						<Table.Cell class="hidden whitespace-nowrap xl:table-cell">
							{#if activity.size}
								<Badge class={cn('h-5 px-1.5 py-0 text-[11px]', sizeBadgeClass(activity.size))}>{activity.size}</Badge>
							{:else}
								<span class="text-muted-foreground">{text.none}</span>
							{/if}
						</Table.Cell>
						<Table.Cell class="hidden whitespace-nowrap text-muted-foreground sm:table-cell"><p class="truncate">{organization?.name ?? text.none}</p></Table.Cell>
						<Table.Cell class="hidden whitespace-nowrap text-muted-foreground xl:table-cell"><p class="truncate">{opportunity?.name ?? text.none}</p></Table.Cell>
						<Table.Cell class="hidden whitespace-nowrap text-right tabular-nums text-muted-foreground md:table-cell">{formatCRMDate(activity.occurredAt.slice(0, 10), currentLocale.value)}</Table.Cell>
						<Table.Cell class="hidden whitespace-nowrap text-right tabular-nums text-muted-foreground xl:table-cell">{activity.calendarEndsAt ? formatCRMDate(activity.calendarEndsAt.slice(0, 10), currentLocale.value) : text.none}</Table.Cell>
						<Table.Cell class="hidden whitespace-nowrap lg:table-cell">
							{#if activity.participantNames.length > 0}
								<PersonChipList names={activity.participantNames} memberIDs={activity.participantIDs} memberEmail={emailOf} />
							{:else}
								<span class="text-muted-foreground">{text.none}</span>
							{/if}
						</Table.Cell>
						<Table.Cell class="whitespace-nowrap pr-6">
							<div class="flex">
								<TaskStatusBadge status={activity.taskStatus} label={taskStatusLabelFrom(text.taskStatuses, activity.taskStatus)} />
							</div>
						</Table.Cell>
					</Table.Row>
				{:else}
					<Table.Row class="hover:bg-transparent"><Table.Cell colspan={10} class="py-10 text-center text-sm text-muted-foreground">{text.noActivities}</Table.Cell></Table.Row>
				{/each}
			</Table.Body>
		</Table.Root>
	</div>
	<div class="border-t bg-card px-3 py-3">
		<ListPaginationFooter
 onPageChange={(page) => (pageIndex = page)}
			totalItems={sortedActivities.length}
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
