<script lang="ts">
	import { page } from '$app/state';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import * as Select from '$lib/components/ui/select';
	import * as Tabs from '$lib/components/ui/tabs';
	import * as UnderlineTabs from '$lib/components/ui/underline-tabs';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { isSupabaseConfigured, supabaseMemberRole } from '$lib/supabase-session';
	import AlertCircleIcon from '@lucide/svelte/icons/alert-circle';
	import CheckCircle2Icon from '@lucide/svelte/icons/check-circle-2';
	import HandshakeIcon from '@lucide/svelte/icons/handshake';
	import LoaderCircleIcon from '@lucide/svelte/icons/loader-circle';
	import PlusIcon from '@lucide/svelte/icons/plus';
	import SearchIcon from '@lucide/svelte/icons/search';
	import XIcon from '@lucide/svelte/icons/x';
	import { onMount } from 'svelte';
	import CRMActivityDetailSheet from './crm-activity-detail-sheet.svelte';
	import CRMActivityTable from './crm-activity-table.svelte';
	import CRMContactEditSheet from './crm-contact-edit-sheet.svelte';
	import CRMContactTable from './crm-contact-table.svelte';
	import CRMDefinitionsEditor from './crm-definitions-editor.svelte';
	import { currentCRMDate, shiftCRMDate } from './crm-date';
	import { crmLabel } from './crm-labels';
	import CRMKPICard from './crm-kpi-card.svelte';
	import CRMOpportunityEditSheet from './crm-opportunity-edit-sheet.svelte';
	import { CRMPageController } from './crm-page-controller.svelte';
	import CRMPipelineBoard from './crm-pipeline-board.svelte';
	import type { CRMPipelineBoardMoveRequest } from './crm-pipeline-board-drag';
	import CRMProgressTable from './crm-progress-table.svelte';
	import CRMQuickCreateMenu from './crm-quick-create-menu.svelte';
	import CRMRecordSheet from './crm-record-sheet.svelte';
	import CRMRelationshipDetailSheet from './crm-relationship-detail-sheet.svelte';
	import CRMRelationshipEditSheet from './crm-relationship-edit-sheet.svelte';
	import CRMRelationshipTable from './crm-relationship-table.svelte';
	import CRMReportDashboard from './crm-report-dashboard.svelte';
	import { buildCRMKPICards } from './crm-kpi';
	import type {
		CRMOrganization,
		CRMOrganizationStatus,
		CRMActivity,
		CRMActivityEditDraft,
		CRMActivityKind,
		CRMContact,
		CRMCreateDraft,
		CRMOpportunity,
		CRMProgressKind,
		CRMRecordKind
	} from './crm-types';
	import {
		organizationMatchesFilters,
		crmOrganizationStatusOptions,
		findOrganizationByID,
		type CRMOrganizationStatusFilter,
		type CRMOrganizationTypeFilter,
		type CRMTab
	} from './crm-view-model';
	import { crmText } from './text';

	type RelationshipView = 'all' | 'mine' | 'attention' | 'recent';
	type PipelineView = 'table' | 'board';
	type PipelineFilter = CRMProgressKind | 'all';
	type ActivityView = CRMActivityKind | 'all';

	const text = createPageText(crmText);
	const controller = new CRMPageController(text);
	const tabItems: Array<{ value: CRMTab; label: string }> = [
		{ value: 'relationships', label: text.relationships },
		{ value: 'contacts', label: text.contactDirectory },
		{ value: 'pipeline', label: text.pipeline },
		{ value: 'activities', label: text.activities },
		{ value: 'reports', label: text.reports },
		{ value: 'definitions', label: text.definitions }
	];
	const relationshipViews: Array<{ value: RelationshipView; label: string }> = [
		{ value: 'all', label: text.allRelationships },
		{ value: 'mine', label: text.myRelationships },
		{ value: 'attention', label: text.needsAttention },
		{ value: 'recent', label: text.recentlyContacted }
	];
	const recentContactThreshold = shiftCRMDate(currentCRMDate(), -30);

	let selectedTab = $state<CRMTab>('relationships');
	let searchQuery = $state('');
	let selectedStatus = $state<CRMOrganizationStatusFilter>('all');
	let selectedType = $state<CRMOrganizationTypeFilter>('all');
	let relationshipView = $state<RelationshipView>('all');
	let activityView = $state<ActivityView>('all');
	let pipelineView = $state<PipelineView>('table');
	let selectedPipeline = $state<PipelineFilter>('all');
	let selectedOrganizationID = $state<string | null>(null);
	let selectedContactID = $state<string | null>(null);
	let selectedOpportunityID = $state<string | null>(null);
	let requestedOpportunityStage = $state<string | undefined>();
	let selectedActivityID = $state<string | null>(null);
	let isOrganizationSheetOpen = $state(false);
	let isRelationshipEditOpen = $state(false);
	let isContactEditOpen = $state(false);
	let isOpportunityEditOpen = $state(false);
	let isActivityEditOpen = $state(false);
	let isRecordSheetOpen = $state(false);
	let createKind = $state<CRMRecordKind>('relationship');
	let createOrganizationID = $state('');
	let feedbackMessage = $state('');
	let isAdmin = $state(false);
	let activityKinds = $derived(controller.activityKindOptions);
	let organizationTypeFilters = $derived<CRMOrganizationTypeFilter[]>(['all', ...controller.organizationTypeOptions]);

	onMount(() => {
		void controller.load(page.data.session?.email ?? '');
		if (isSupabaseConfigured()) void supabaseMemberRole().then((role) => (isAdmin = role === 'admin'));
	});

	$effect(() => {
		if (selectedPipeline !== 'all' && !controller.pipelines.some((pipeline) => pipeline.pipeline === selectedPipeline)) {
			selectedPipeline = 'all';
		}
		if (selectedPipeline === 'all') pipelineView = 'table';
		selectedOrganizationID ??= controller.organizations[0]?.id ?? null;
		selectedContactID ??= controller.contacts[0]?.id ?? null;
		selectedOpportunityID ??= controller.opportunities[0]?.id ?? null;
		selectedActivityID ??= controller.activities[0]?.id ?? null;
	});

	let filteredOrganizations = $derived(controller.organizations.filter((organization) => {
		if (!organizationMatchesFilters(organization, searchQuery, selectedStatus, selectedType)) return false;
		if (relationshipView === 'mine') return organization.ownerName === controller.currentOwnerName;
		if (relationshipView === 'attention') return organization.status === 'paused' || organization.importance === 'low';
		if (relationshipView === 'recent') return organization.lastContactDate >= recentContactThreshold;
		return true;
	}));
	let filteredContacts = $derived(controller.contacts.filter((contact) => {
		const organization = findOrganizationByID(controller.organizations, contact.organizationID);
		const query = searchQuery.trim().toLowerCase();
		return query === '' || [contact.name, contact.title, contact.email, contact.phone ?? '', contact.note ?? '', organization?.name ?? '']
			.some((value) => value.toLowerCase().includes(query));
	}));
	let filteredActivities = $derived(controller.activities.filter((activity) => {
		if (activityView !== 'all' && activity.kind !== activityView) return false;
		const organization = findOrganizationByID(controller.organizations, activity.organizationID);
		const query = searchQuery.trim().toLowerCase();
		return query === '' || [activity.title, activity.summary, organization?.name ?? ''].some((value) => value.toLowerCase().includes(query));
	}));
	let pipelineOpportunities = $derived(selectedPipeline === 'all'
		? controller.opportunities
		: controller.opportunities.filter((opportunity) => (opportunity.pipeline ?? opportunity.kind) === selectedPipeline));
	let selectedPipelineStages = $derived(selectedPipeline === 'all' ? [] : controller.stagesFor(selectedPipeline));
	let selectedOrganization = $derived(selectedOrganizationID ? findOrganizationByID(controller.organizations, selectedOrganizationID) : undefined);
	let selectedContact = $derived(selectedContactID ? controller.contacts.find((contact) => contact.id === selectedContactID) : undefined);
	let selectedOpportunity = $derived(selectedOpportunityID ? controller.opportunities.find((opportunity) => opportunity.id === selectedOpportunityID) : undefined);
	let selectedActivity = $derived(selectedActivityID ? controller.activities.find((activity) => activity.id === selectedActivityID) : undefined);
	let kpiCards = $derived(buildCRMKPICards(controller.organizations, controller.opportunities, controller.nextActions, controller.stages, text));

	function openOrganization(organizationID: string): void {
		selectedOrganizationID = organizationID;
		isOrganizationSheetOpen = true;
	}

	function openOrganizationEdit(organizationID: string): void {
		selectedOrganizationID = organizationID;
		isOrganizationSheetOpen = false;
		isRelationshipEditOpen = true;
	}

	function openOpportunityEdit(opportunityID: string): void {
		selectedOpportunityID = opportunityID;
		requestedOpportunityStage = undefined;
		isOpportunityEditOpen = true;
	}

	function openContactEdit(contactID: string): void {
		selectedContactID = contactID;
		isContactEditOpen = true;
	}

	function openRelationshipContactEdit(contactID: string): void {
		isRelationshipEditOpen = false;
		openContactEdit(contactID);
	}

	function openRelationshipContactCreate(organizationID: string): void {
		isRelationshipEditOpen = false;
		createKind = 'contact';
		createOrganizationID = organizationID;
		isRecordSheetOpen = true;
	}

	function openActivityEdit(activityID: string): void {
		selectedActivityID = activityID;
		isActivityEditOpen = true;
	}

	function openCreateSheet(kind: CRMRecordKind): void {
		createKind = kind;
		createOrganizationID = '';
		isRecordSheetOpen = true;
	}

	function selectPipeline(value: string): void {
		selectedPipeline = value as PipelineFilter;
		if (selectedPipeline === 'all') pipelineView = 'table';
	}

	async function handleCreate(draft: CRMCreateDraft): Promise<void> {
		await controller.create(draft);
		selectedTab = draft.kind === 'relationship' ? 'relationships' : draft.kind === 'contact' ? 'contacts' : draft.kind === 'progress' ? 'pipeline' : 'activities';
		feedbackMessage = text.savedToService;
	}

	async function saveOrganization(organization: CRMOrganization): Promise<void> {
		await controller.saveOrganization(organization);
		feedbackMessage = text.savedToService;
	}

	async function archiveOrganization(organizationID: string): Promise<void> {
		await controller.archiveOrganization(organizationID);
		isRelationshipEditOpen = false;
		feedbackMessage = text.archivedFromService;
	}

	async function saveContact(contact: CRMContact): Promise<void> {
		await controller.saveContact(contact);
		feedbackMessage = text.savedToService;
	}

	async function saveOpportunity(opportunity: CRMOpportunity): Promise<void> {
		await controller.saveOpportunity(opportunity);
		feedbackMessage = text.savedToService;
	}

	async function archiveOpportunity(opportunityID: string): Promise<void> {
		await controller.archiveOpportunity(opportunityID);
		isOpportunityEditOpen = false;
		feedbackMessage = text.archivedFromService;
	}

	async function saveActivity(activity: CRMActivity, draft: CRMActivityEditDraft): Promise<void> {
		await controller.saveActivity(activity, draft);
		feedbackMessage = text.savedToService;
	}

	function moveOpportunity(request: CRMPipelineBoardMoveRequest): void {
		const opportunity = controller.opportunities.find((candidate) => candidate.id === request.opportunityID);
		const outcome = controller.stages.find((stage) =>
			stage.pipeline === (opportunity?.pipeline ?? opportunity?.kind) && stage.stage === request.targetStage
		)?.outcome;
		if (outcome === 'won' || outcome === 'lost') {
			selectedOpportunityID = request.opportunityID;
			requestedOpportunityStage = request.targetStage;
			isOpportunityEditOpen = true;
			return;
		}
		void controller.moveOpportunity(request)
			.then(() => {
				feedbackMessage = text.savedToService;
			})
			.catch(() => {
				feedbackMessage = '';
			});
	}
</script>

<svelte:head><title>{text.pageTitle}</title></svelte:head>

<main data-crm-ready={!controller.isLoading && !controller.errorMessage} class="grid min-h-[calc(100svh-48px)] w-full content-start gap-4 px-4 pb-32 pt-4 sm:px-6 sm:pb-36 lg:px-8">
	<header class="flex flex-wrap items-center gap-3 border-b pb-3" data-crm-toolbar>
		<div class="flex shrink-0 items-center gap-2">
			<span class="flex size-8 shrink-0 items-center justify-center rounded-md bg-foreground text-background"><HandshakeIcon class="size-4" /></span>
			<h1 class="text-xl font-semibold">{text.title}</h1>
		</div>
		<label class="relative order-last min-w-0 basis-full lg:order-none lg:ml-auto lg:max-w-md lg:flex-1">
			<SearchIcon class="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
			<Input bind:value={searchQuery} class="h-8 pl-9" placeholder={text.searchPlaceholder} disabled={controller.isLoading} />
		</label>
		<div class="ml-auto flex shrink-0 items-center gap-2 lg:ml-0"><CRMQuickCreateMenu {text} onCreate={openCreateSheet} /></div>
	</header>

	{#if feedbackMessage}
		<div role="status" class="flex items-center gap-2 rounded-md border bg-muted/40 px-3 py-2 text-sm"><CheckCircle2Icon class="size-4 text-primary" /><span>{feedbackMessage}</span><Button type="button" variant="ghost" size="icon-sm" class="ml-auto" aria-label={text.cancel} onclick={() => (feedbackMessage = '')}><XIcon /></Button></div>
	{/if}
	{#if controller.errorMessage}
		<div role="alert" class="flex flex-wrap items-center gap-3 rounded-md border border-destructive/30 bg-destructive/10 px-4 py-3 text-sm text-destructive">
			<AlertCircleIcon class="size-4" /><span class="flex-1">{controller.errorMessage}</span>
			<Button type="button" variant="outline" size="sm" onclick={() => controller.load(page.data.session?.email ?? '')}>{text.retry}</Button>
		</div>
	{/if}

	{#if controller.isLoading}
		<div role="status" class="flex min-h-64 items-center justify-center gap-2 rounded-md border text-sm text-muted-foreground"><LoaderCircleIcon class="size-4 animate-spin" />{text.loading}</div>
	{:else if !controller.errorMessage}
		<section class="grid grid-cols-1 items-stretch gap-3 sm:grid-cols-2 lg:grid-cols-4" data-crm-metrics>
			{#each kpiCards as card (card.id)}<CRMKPICard {card} />{/each}
		</section>

		<UnderlineTabs.Root bind:value={selectedTab} class="min-w-0 gap-4">
			<UnderlineTabs.List class="overflow-x-clip">{#each tabItems as tab (tab.value)}<UnderlineTabs.Trigger value={tab.value}>{tab.label}</UnderlineTabs.Trigger>{/each}</UnderlineTabs.List>

			<UnderlineTabs.Content value="relationships" class="grid min-w-0 gap-4 pb-24">
				<div class="flex min-w-0 flex-col gap-3 rounded-md border bg-card p-3 lg:flex-row lg:items-center lg:justify-end">
					<Select.Root type="single" value={relationshipView} onValueChange={(value) => (relationshipView = value as RelationshipView)}><Select.Trigger class="w-full lg:w-44">{relationshipViews.find((view) => view.value === relationshipView)?.label}</Select.Trigger><Select.Content>{#each relationshipViews as view (view.value)}<Select.Item value={view.value} label={view.label}>{view.label}</Select.Item>{/each}</Select.Content></Select.Root>
					<Select.Root type="single" value={selectedStatus} onValueChange={(value) => (selectedStatus = value as CRMOrganizationStatusFilter)}><Select.Trigger class="w-full lg:w-40">{selectedStatus === 'all' ? text.allStatuses : text.organizationStatuses[selectedStatus as CRMOrganizationStatus]}</Select.Trigger><Select.Content>{#each crmOrganizationStatusOptions as status (status)}<Select.Item value={status} label={status === 'all' ? text.allStatuses : text.organizationStatuses[status]}>{status === 'all' ? text.allStatuses : text.organizationStatuses[status]}</Select.Item>{/each}</Select.Content></Select.Root>
					<Select.Root type="single" value={selectedType} onValueChange={(value) => (selectedType = value as CRMOrganizationTypeFilter)}><Select.Trigger class="w-full lg:w-40">{selectedType === 'all' ? text.allTypes : crmLabel(text.organizationTypes, selectedType)}</Select.Trigger><Select.Content>{#each organizationTypeFilters as organizationType (organizationType)}<Select.Item value={organizationType} label={organizationType === 'all' ? text.allTypes : crmLabel(text.organizationTypes, organizationType)}>{organizationType === 'all' ? text.allTypes : crmLabel(text.organizationTypes, organizationType)}</Select.Item>{/each}</Select.Content></Select.Root>
					<Button type="button" size="sm" onclick={() => openCreateSheet('relationship')}><PlusIcon data-icon="inline-start" />{text.newRelationship}</Button>
				</div>
				<CRMRelationshipTable organizations={filteredOrganizations} contacts={controller.contacts} {text} {openOrganization} />
			</UnderlineTabs.Content>

			<UnderlineTabs.Content value="contacts" class="grid min-w-0 gap-4 pb-24">
				<div class="flex min-w-0 flex-col gap-3 rounded-md border bg-card p-3 sm:flex-row sm:items-center sm:justify-end">
					<Button type="button" size="sm" onclick={() => openCreateSheet('contact')}><PlusIcon data-icon="inline-start" />{text.newContact}</Button>
				</div>
				<CRMContactTable contacts={filteredContacts} organizations={controller.organizations} {text} onEdit={openContactEdit} />
			</UnderlineTabs.Content>

			<UnderlineTabs.Content value="pipeline" class="grid min-w-0 gap-4 pb-24">
				<div class="flex flex-wrap items-center justify-between gap-3 rounded-md border bg-card p-3">
					<div class="flex flex-wrap items-center gap-3">
						<Select.Root type="single" value={selectedPipeline} onValueChange={selectPipeline}><Select.Trigger class="w-44" aria-label={text.progressKind}>{selectedPipeline === 'all' ? text.allProgressKinds : controller.pipelines.find((pipeline) => pipeline.pipeline === selectedPipeline)?.label ?? selectedPipeline}</Select.Trigger><Select.Content><Select.Item value="all" label={text.allProgressKinds}>{text.allProgressKinds}</Select.Item>{#each controller.pipelines as pipeline (pipeline.pipeline)}<Select.Item value={pipeline.pipeline} label={pipeline.label}>{pipeline.label}</Select.Item>{/each}</Select.Content></Select.Root>
							<Tabs.Root bind:value={pipelineView} aria-label={text.pipeline}><Tabs.List><Tabs.Trigger value="table">{text.tableView}</Tabs.Trigger><Tabs.Trigger value="board" disabled={selectedPipeline === 'all'}>{text.boardView}</Tabs.Trigger></Tabs.List></Tabs.Root>
					</div>
					<Button type="button" size="sm" onclick={() => openCreateSheet('progress')}><PlusIcon data-icon="inline-start" />{text.newOpportunity}</Button>
				</div>
				{#if pipelineView === 'table'}
					<CRMProgressTable opportunities={pipelineOpportunities} organizations={controller.organizations} nextActions={controller.nextActions} {text} onEdit={openOpportunityEdit} />
				{:else}
					<CRMPipelineBoard opportunities={pipelineOpportunities} organizations={controller.organizations} nextActions={controller.nextActions} stages={selectedPipelineStages} {text} onMove={moveOpportunity} />
				{/if}
			</UnderlineTabs.Content>

			<UnderlineTabs.Content value="activities" class="grid min-w-0 gap-4 pb-24">
				<div class="flex min-w-0 flex-col gap-3 rounded-md border bg-card p-3 sm:flex-row sm:items-center sm:justify-end">
					<Select.Root type="single" value={activityView} onValueChange={(value) => (activityView = value as ActivityView)}><Select.Trigger class="w-full sm:w-44">{activityView === 'all' ? text.allActivityKinds : crmLabel(text.activityKinds, activityView)}</Select.Trigger><Select.Content><Select.Item value="all" label={text.allActivityKinds}>{text.allActivityKinds}</Select.Item>{#each activityKinds as activityKind (activityKind)}<Select.Item value={activityKind} label={crmLabel(text.activityKinds, activityKind)}>{crmLabel(text.activityKinds, activityKind)}</Select.Item>{/each}</Select.Content></Select.Root>
					<Button type="button" size="sm" onclick={() => openCreateSheet('activity')}><PlusIcon data-icon="inline-start" />{text.logActivity}</Button>
				</div>
				<CRMActivityTable activities={filteredActivities} organizations={controller.organizations} {text} onEdit={openActivityEdit} />
			</UnderlineTabs.Content>

			<UnderlineTabs.Content value="reports" class="min-w-0 pb-24"><CRMReportDashboard organizations={controller.organizations} opportunities={controller.opportunities} nextActions={controller.nextActions} stages={controller.stages} {text} onOpenOrganization={openOrganization} /></UnderlineTabs.Content>
			<UnderlineTabs.Content value="definitions" class="min-w-0 pb-24"><CRMDefinitionsEditor vocabulary={controller.vocabulary} {isAdmin} isSaving={controller.isSaving} errorMessage={controller.errorMessage} onSave={(vocabulary) => controller.saveVocabulary(vocabulary)} onDelete={(_request, vocabulary) => controller.saveVocabulary(vocabulary)} /></UnderlineTabs.Content>
		</UnderlineTabs.Root>
	{/if}
</main>

<CRMRelationshipDetailSheet bind:open={isOrganizationSheetOpen} organization={selectedOrganization} contacts={controller.contacts} opportunities={controller.opportunities} activities={controller.activities} {text} onEdit={openOrganizationEdit} />
<CRMRelationshipEditSheet bind:open={isRelationshipEditOpen} organization={selectedOrganization} contacts={controller.contacts} organizationTypeOptions={controller.organizationTypeOptions} people={controller.people} groups={controller.groups} {text} onSave={saveOrganization} onArchive={archiveOrganization} onEditContact={openRelationshipContactEdit} onCreateContact={openRelationshipContactCreate} />
<CRMContactEditSheet bind:open={isContactEditOpen} contact={selectedContact} organizations={controller.organizations} opportunities={controller.opportunities} {text} onSave={saveContact} />
<CRMOpportunityEditSheet bind:open={isOpportunityEditOpen} opportunity={selectedOpportunity} organizations={controller.organizations} contacts={controller.contacts} pipelines={controller.pipelines} stages={controller.stages} lostReasons={controller.lostReasons} businessOptions={controller.businessOptions} people={controller.people} groups={controller.groups} requestedStage={requestedOpportunityStage} {text} onSave={saveOpportunity} onArchive={archiveOpportunity} />
<CRMActivityDetailSheet bind:open={isActivityEditOpen} activity={selectedActivity} organizations={controller.organizations} opportunities={controller.opportunities} contacts={controller.contacts} businessOptions={controller.businessOptions} activityKindOptions={controller.activityKindOptions} people={controller.people} groups={controller.groups} {text} onSave={saveActivity} />
<CRMRecordSheet bind:open={isRecordSheetOpen} initialKind={createKind} initialOrganizationID={createOrganizationID} organizations={controller.organizations} contacts={controller.contacts} opportunities={controller.opportunities} pipelines={controller.pipelines} stages={controller.stages} lostReasons={controller.lostReasons} businessOptions={controller.businessOptions} organizationTypeOptions={controller.organizationTypeOptions} activityKindOptions={controller.activityKindOptions} defaultOwnerPersonID={controller.currentOwnerPersonID} people={controller.people} groups={controller.groups} {text} onCreate={handleCreate} />
