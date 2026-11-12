import type { SupabaseClient } from '@supabase/supabase-js';
import { attendanceAdd, attendanceDelete, attendanceList, attendanceUpdate } from './attendance-tools';
import { currentAttendance } from './current-attendance';
import { teamAttendance, legacyTeamAttendance } from './team-attendance';
import { exchangeRatesOf } from '$lib/server/configured-exchange-rates';
import { recordContextOf, type PeopleRead, type RecordContext, type ToolPlace } from './company';
import type { TaskChangeAnnouncer } from './task-change';
import type { AttendanceAnnouncer } from './attendance-announcement';
import {
	attendanceLeavePolicyGet,
	attendanceLeavePolicySet,
	attendanceWorkPolicyGet,
	attendanceWorkPolicySet,
	companyHolidayAdd,
	companyHolidayDelete,
	companyHolidayList,
	companyHolidayUpdate,
	companyInfoGet,
	companyInfoSet,
	companySettingsGet,
	companySettingsUpdate
} from './company-tools';
import { companyImageUpload } from './service-files';
import { companyCreditsGet } from './company-credits';
import { crmActivityList, crmActivitySave } from './crm-activity-tools';
import {
	crmContactAdd,
	crmContactArchive,
	crmContactList,
	crmContactUpdate,
	crmOpportunityAdd,
	crmOpportunityArchive,
	crmOpportunityList,
	crmOpportunityMove,
	crmOpportunityUpdate,
	crmOrganizationAdd,
	crmOrganizationArchive,
	crmOrganizationList,
	crmOrganizationUpdate,
	crmVocabularyGet,
	crmVocabularySet
} from './crm-tools';
import { crmRecordImport } from './crm-import-tools';
import { crmImportRecordsOf, crmRecordImportPreview } from './crm-import-file';
import type { CarriedWorkspaceFile } from '../carried-files';
import {
	companyDocumentDownload,
	companyDocumentList,
	companyDocumentRegister,
	companyDocumentRecall,
	companyDocumentSearch,
	companyDocumentUpdate,
	companyDocumentUpload,
	companyMetricList,
	companyMetricRecord,
	companyRecordAdd,
	companyRecordDelete,
	companyRecordList,
	companyRecordUpdate
} from './company-ledger-tools';
import { companyEquityAdd, companyEquityDelete, companyEquityList } from './company-equity-tools';
import {
	eventAdd,
	eventDelete,
	eventList,
	eventUpdate
} from './event-tools';
import { calendarSubscriptionAdd, calendarSubscriptionDelete, calendarSubscriptionGet } from './calendar-subscription';
import {
	conversationMute,
	conversationUnmute,
	notificationSettingsGet,
	notificationSettingsSet
} from './notification-tools';
import { channelOrderGet, channelOrderSet, channelPin, channelUnpin } from './channel-order-tools';
import {
	leaveBalance,
	leaveDecide,
	leaveDelete,
	leaveGrantSet,
	leaveList,
	leaveRequest,
	leaveReturnEarly,
	leaveUpdate
} from './leave-tools';
import { personInvite, personList, personPasswordReset, personUpdate, personWithdraw } from './people-tools';
import { companyDataRoomGet, companyDataRoomCategorySet, companyDataRoomFieldSet, companyDataRoomFieldNameSet,
	companyDataRoomShareCreate, companyDataRoomShareRevoke,
	companyDataRoomLinksGet, companyDataRoomLinkCreate, companyDataRoomLinkRevoke,
	companyDataRoomInvitationAccept, companyDataRoomInvitationSend } from './data-room';
import { taskInvite, taskInvitationDelete, taskInvitationList } from './task-invitation';
import {
	companyCircleChannelAdd, companyCircleChannelDelete, companyCircleList, companyCircleMemberSet, companyCircleSet
} from './circle';
import { taskAdd, taskDelete, taskList, taskBoard, taskUpdate, taskVocabularySet } from './task-tools';
import { teamAdd, teamDelete, teamList, teamUpdate } from './team-tools';
import {
	companyHolidayDeleteTarget,
	companyRecordDeleteTarget,
	crmContactArchiveTarget,
	crmOpportunityArchiveTarget,
	crmOrganizationArchiveTarget,
	eventDeleteTarget,
	personTarget,
	taskDeleteTarget,
	type PreviewedTarget
} from './preview';
import { leavesTaskLabelsUndecided, type TaskLabelDecider } from './task-labels';
import { answererOfTool, toolNamesAnsweredBy } from '../catalog';
import { capabilityToolResultSchema, legacyAttendanceListResultSchema } from '../catalog/tools';
import { readWithNullAsAbsent } from '../null-as-absent';
import { sentencesOfSchemaRefusal } from '../schema-sentences';
import { Refusal, answerOfRefusal } from '../refusal';
import { attendanceAddInputSchema, attendanceCurrentGetInputSchema, attendanceDeleteInputSchema, attendanceListInputSchema, attendanceUpdateInputSchema, calendarAddInputSchema, calendarDeleteInputSchema, calendarListInputSchema, calendarUpdateInputSchema, leaveBalanceInputSchema, leaveDecideInputSchema, leaveDeleteInputSchema, leaveGrantSetInputSchema, leaveListInputSchema, leaveRequestInputSchema, leaveReturnEarlyInputSchema, leaveUpdateInputSchema, taskAddInputSchema, taskBoardInputSchema, taskDeleteInputSchema, taskListInputSchema, taskUpdateInputSchema } from '../catalog/tools';
import { taskVocabularySetInputSchema } from '../catalog/labels';
import { taskInvitationDeleteInputSchema, taskInvitationListInputSchema, taskInviteInputSchema } from '../catalog/task-invitation';
import { calendarSubscriptionInputSchema } from '../catalog/calendar-subscription';
import { personInviteInputSchema, personListInputSchema, personPasswordResetInputSchema, personUpdateInputSchema, personWithdrawInputSchema, teamAddInputSchema, teamDeleteInputSchema, teamListInputSchema, teamUpdateInputSchema } from '../catalog/people';
import { attendanceTeamPageInputSchema } from '$lib/attendance/team-page';
import { attendanceLeavePolicyGetInputSchema, attendanceLeavePolicySetInputSchema, attendanceWorkPolicyGetInputSchema, attendanceWorkPolicySetInputSchema, companyHolidayAddInputSchema, companyHolidayDeleteInputSchema, companyHolidayListInputSchema, companyHolidayUpdateInputSchema, companySettingsGetInputSchema, companySettingsUpdateInputSchema } from '../catalog/settings';
import { companyCreditsGetInputSchema } from '../catalog/credits';
import { companyDocumentDownloadInputSchema, companyDocumentListInputSchema, companyDocumentRegisterInputSchema, companyDocumentSearchInputSchema, companyDocumentUpdateInputSchema, companyDocumentUploadInputSchema, companyEquityAddInputSchema, companyEquityDeleteInputSchema, companyEquityListInputSchema, companyImageUploadInputSchema, companyInfoGetInputSchema, companyInfoSetInputSchema, companyMetricListInputSchema, companyMetricRecordInputSchema, companyRecordAddInputSchema, companyRecordDeleteInputSchema, companyRecordListInputSchema, companyRecordUpdateInputSchema } from '../catalog/company';
import { recallSourceInputSchema } from '../catalog/protocol';
import { circleChannelAddInputSchema, circleChannelDeleteInputSchema, circleListInputSchema, circleMemberSetInputSchema, circleSetInputSchema } from '../catalog/circle';
import { dataRoomCategorySetInputSchema, dataRoomFieldNameSetInputSchema, dataRoomFieldSetInputSchema, dataRoomGetInputSchema, dataRoomInvitationAcceptInputSchema, dataRoomInvitationSendInputSchema, dataRoomLinksGetInputSchema, dataRoomShareCreateInputSchema, dataRoomShareRevokeInputSchema } from '../catalog/data-room';
import { dataRoomLinkInputSchema, dataRoomLinkRevokeSchema } from '$lib/data-room/links';
import { conversationMuteInputSchema, notificationSettingsGetInputSchema, notificationSettingsSetInputIntentSchema } from '../catalog/notifications';
import { channelOrderGetInputSchema, channelOrderSetInputSchema, channelPinInputSchema } from '../catalog/channel-order';
import { crmActivityListInputSchema, crmActivitySaveInputSchema, crmContactAddInputSchema, crmContactArchiveInputSchema, crmContactListInputSchema, crmContactUpdateInputSchema, crmOpportunityAddInputSchema, crmOpportunityArchiveInputSchema, crmOpportunityListInputSchema, crmOpportunityMoveInputSchema, crmOpportunityUpdateInputSchema, crmOrganizationAddInputSchema, crmOrganizationArchiveInputSchema, crmOrganizationListInputSchema, crmOrganizationUpdateInputSchema, crmRecordImportInputSchema, crmVocabularyGetInputSchema, crmVocabularySetInputSchema } from '../catalog/crm';
import type { z } from 'zod';
import { inputOfTheCall, parsedToolInput } from '../tool-input';

type CallCarried = {
	files: CarriedWorkspaceFile[];
	place?: ToolPlace;
	announceTaskChange?: TaskChangeAnnouncer;
	announceAttendance?: AttendanceAnnouncer;
};
type RecordToolRun<Input> = (context: RecordContext, input: Input, carried: CallCarried) => unknown;
type RecordToolPreview<Input> = (
	context: RecordContext,
	input: Input,
	files: CarriedWorkspaceFile[]
) => Promise<PreviewedTarget | null>;
type ReadsPeople<Input> = (input: Input) => PeopleRead;
type RecordReach = { caller: SupabaseClient; context: () => Promise<RecordContext> };
type BoundRecordTool = {
	peopleRead: PeopleRead;
	run: (reach: RecordReach, carried: CallCarried) => unknown;
	preview?: (context: RecordContext, files: CarriedWorkspaceFile[]) => Promise<PreviewedTarget | null>;
};
type RecordTool = { inputSchema: z.ZodType; boundTo: (asked: unknown) => BoundRecordTool };

function recordTool<Input>(
	inputSchema: z.ZodType<Input>,
	readsPeople: ReadsPeople<Input>,
	run: RecordToolRun<Input>,
	preview?: RecordToolPreview<Input>
): RecordTool {
	return {
		inputSchema,
		boundTo: (asked) => {
			const input = parsedToolInput(inputSchema, asked);
			const runWith: BoundRecordTool['run'] = async (reach, carried) => run(await reach.context(), input, carried);
			const peopleRead = readsPeople(input);
			if (!preview) return { peopleRead, run: runWith };
			return { peopleRead, run: runWith, preview: (context, files) => preview(context, input, files) };
		}
	};
}

function callerTool<Input>(inputSchema: z.ZodType<Input>, run: (caller: SupabaseClient, input: Input) => unknown): RecordTool {
	return {
		inputSchema,
		boundTo: (asked) => {
			const input = parsedToolInput(inputSchema, asked);
			return { peopleRead: 'nobody', run: (reach) => run(reach.caller, input) };
		}
	};
}

const everyone = (): PeopleRead => 'everyone';

const nobody = (): PeopleRead => 'nobody';

const exactPersonID = /^[0-9a-f]{8}-(?:[0-9a-f]{4}-){3}[0-9a-f]{12}$/i;

function thePersonNamedByIDOrEveryone(input: { personHints?: string[] }): PeopleRead {
	const hints = input.personHints ?? [];
	if (hints.length === 1 && exactPersonID.test(hints[0])) return { personIDs: hints };
	return 'everyone';
}

function peopleAnAttendancePageReads(input: z.infer<typeof attendanceListInputSchema>): PeopleRead {
	const isTheHandWrittenPage = input.handWrittenOnly === true && input.pageOffset !== undefined;
	if (isTheHandWrittenPage && input.scope === 'all' && !input.personHints) return 'nobody';
	return thePersonNamedByIDOrEveryone(input);
}

function peopleAPersonHintReads(input: { personHint?: string }): PeopleRead {
	return input.personHint ? 'everyone' : 'nobody';
}

function peopleAnAttendanceChangeReads(input: { undoOnly?: boolean }): PeopleRead {
	return input.undoOnly === true ? 'nobody' : 'everyone';
}

function placeTheAppAnswersAt(name: string, place: ToolPlace | undefined): ToolPlace {
	if (!place) throw new Error(`${name} names an address in the app, and this call did not say where the app answers`);
	return place;
}

const toolsOverTheRecord: Record<string, RecordTool> = {
	task_add: recordTool(taskAddInputSchema, everyone, (context, input, { announceTaskChange }) => taskAdd(context, input, announceTaskChange)),
	task_update: recordTool(taskUpdateInputSchema, everyone, (context, input, { announceTaskChange }) => taskUpdate(context, input, announceTaskChange)),
	task_list: recordTool(taskListInputSchema, everyone, (context, input) => taskList(context, input)),
	task_board_get: recordTool(taskBoardInputSchema, everyone, (context, input) => taskBoard(context, input)),
	task_delete: recordTool(taskDeleteInputSchema, everyone, (context, input) => taskDelete(context, input), taskDeleteTarget),
	task_vocabulary_set: recordTool(taskVocabularySetInputSchema, everyone, (context, input) => taskVocabularySet(context, input)),
	task_invite: recordTool(taskInviteInputSchema, nobody, (context, input, { place }) => taskInvite(context, input, placeTheAppAnswersAt('task_invite', place))),
	task_invitation_list: recordTool(taskInvitationListInputSchema, nobody, (context) => taskInvitationList(context)),
	task_invitation_delete: recordTool(taskInvitationDeleteInputSchema, nobody, (context, input) => taskInvitationDelete(context, input)),
	event_add: recordTool(calendarAddInputSchema, everyone, (context, input) => eventAdd(context, input)),
	event_update: recordTool(calendarUpdateInputSchema, everyone, (context, input) => eventUpdate(context, input)),
	event_list: recordTool(calendarListInputSchema, everyone, (context, input) => eventList(context, input)),
	event_delete: recordTool(calendarDeleteInputSchema, everyone, (context, input) => eventDelete(context, input), eventDeleteTarget),
	calendar_subscription_get: recordTool(calendarSubscriptionInputSchema, nobody, (context) => calendarSubscriptionGet(context)),
	calendar_subscription_add: recordTool(calendarSubscriptionInputSchema, nobody, (context, _input, { place }) =>
		calendarSubscriptionAdd(context, placeTheAppAnswersAt('calendar_subscription_add', place))),
	calendar_subscription_delete: recordTool(calendarSubscriptionInputSchema, nobody, (context) => calendarSubscriptionDelete(context)),
	person_list: recordTool(personListInputSchema, everyone, (context, input) => personList(context, input)),
	person_update: recordTool(personUpdateInputSchema, everyone, (context, input) => personUpdate(context, input)),
	person_invite: recordTool(personInviteInputSchema, everyone, (context, input) => personInvite(context, input)),
	person_withdraw: recordTool(personWithdrawInputSchema, everyone, (context, input) => personWithdraw(context, input), personTarget),
	person_password_reset: recordTool(personPasswordResetInputSchema, everyone, (context, input) => personPasswordReset(context, input), personTarget),
	team_list: recordTool(teamListInputSchema, everyone, (context) => teamList(context)),
	team_add: recordTool(teamAddInputSchema, everyone, (context, input) => teamAdd(context, input)),
	team_update: recordTool(teamUpdateInputSchema, everyone, (context, input) => teamUpdate(context, input)),
	team_delete: recordTool(teamDeleteInputSchema, everyone, (context, input) => teamDelete(context, input)),
	leave_list: recordTool(leaveListInputSchema, thePersonNamedByIDOrEveryone, (context, input) => leaveList(context, input)),
	leave_balance: recordTool(leaveBalanceInputSchema, everyone, (context, input) => leaveBalance(context, input)),
	leave_request: recordTool(leaveRequestInputSchema, everyone, (context, input, { announceAttendance }) => leaveRequest(context, input, announceAttendance)),
	leave_update: recordTool(leaveUpdateInputSchema, everyone, (context, input) => leaveUpdate(context, input)),
	leave_delete: recordTool(leaveDeleteInputSchema, everyone, (context, input) => leaveDelete(context, input)),
	leave_decide: recordTool(leaveDecideInputSchema, everyone, (context, input) => leaveDecide(context, input)),
	leave_grant_set: recordTool(leaveGrantSetInputSchema, everyone, (context, input) => leaveGrantSet(context, input)),
	leave_return_early: recordTool(leaveReturnEarlyInputSchema, everyone, (context, input, { announceAttendance }) => leaveReturnEarly(context, input, announceAttendance)),
	attendance_list: recordTool(attendanceListInputSchema, peopleAnAttendancePageReads, async (context, input) => legacyAttendanceListResultSchema.parse(await attendanceList(context, input))),
	attendance_changes_page_get: recordTool(attendanceListInputSchema, peopleAnAttendancePageReads, (context, input) => attendanceList(context, input)),
	attendance_current_get: callerTool(attendanceCurrentGetInputSchema, (caller) => currentAttendance(caller)),
	attendance_team_page_get: callerTool(attendanceTeamPageInputSchema, legacyTeamAttendance),
	attendance_team_dashboard_get: callerTool(attendanceTeamPageInputSchema, teamAttendance),
	attendance_add: recordTool(attendanceAddInputSchema, peopleAPersonHintReads, (context, input, { announceAttendance }) => attendanceAdd(context, input, announceAttendance)),
	attendance_update: recordTool(attendanceUpdateInputSchema, peopleAnAttendanceChangeReads, (context, input) => attendanceUpdate(context, input)),
	attendance_delete: recordTool(attendanceDeleteInputSchema, peopleAnAttendanceChangeReads, (context, input) => attendanceDelete(context, input)),
	attendance_work_policy_get: recordTool(attendanceWorkPolicyGetInputSchema, everyone, (context) => attendanceWorkPolicyGet(context)),
	attendance_work_policy_set: recordTool(attendanceWorkPolicySetInputSchema, everyone, (context, input) => attendanceWorkPolicySet(context, input)),
	attendance_leave_policy_get: recordTool(attendanceLeavePolicyGetInputSchema, everyone, (context) => attendanceLeavePolicyGet(context)),
	attendance_leave_policy_set: recordTool(attendanceLeavePolicySetInputSchema, everyone, (context, input) => attendanceLeavePolicySet(context, input)),
	company_settings_get: recordTool(companySettingsGetInputSchema, nobody, (context, input) => companySettingsGet(context, input)),
	company_settings_update: recordTool(companySettingsUpdateInputSchema, everyone, (context, input) => companySettingsUpdate(context, input)),
	company_credits_get: recordTool(companyCreditsGetInputSchema, everyone, (context) => companyCreditsGet(context)),
	company_info_get: recordTool(companyInfoGetInputSchema, everyone, (context, input) => companyInfoGet(context, input)),
	company_info_set: recordTool(companyInfoSetInputSchema, everyone, (context, input) => companyInfoSet(context, input)),
	company_image_upload: recordTool(companyImageUploadInputSchema, everyone, (context, input) => companyImageUpload(context, input)),
	company_holiday_list: recordTool(companyHolidayListInputSchema, nobody, (context, input) => companyHolidayList(context, input)),
	company_holiday_add: recordTool(companyHolidayAddInputSchema, everyone, (context, input) => companyHolidayAdd(context, input)),
	company_holiday_update: recordTool(companyHolidayUpdateInputSchema, everyone, (context, input) => companyHolidayUpdate(context, input)),
	company_holiday_delete: recordTool(companyHolidayDeleteInputSchema, everyone, (context, input) => companyHolidayDelete(context, input), companyHolidayDeleteTarget),
	company_metric_record: recordTool(companyMetricRecordInputSchema, everyone, (context, input) => companyMetricRecord(context, input)),
	company_metric_list: recordTool(companyMetricListInputSchema, everyone, (context, input) => companyMetricList(context, input)),
	company_record_add: recordTool(companyRecordAddInputSchema, everyone, (context, input) => companyRecordAdd(context, input)),
	company_record_list: recordTool(companyRecordListInputSchema, everyone, (context, input) => companyRecordList(context, input)),
	company_record_update: recordTool(companyRecordUpdateInputSchema, everyone, (context, input) => companyRecordUpdate(context, input)),
	company_record_delete: recordTool(companyRecordDeleteInputSchema, everyone, (context, input) => companyRecordDelete(context, input), companyRecordDeleteTarget),
	company_equity_add: recordTool(companyEquityAddInputSchema, everyone, (context, input) => companyEquityAdd(context, input)),
	company_equity_list: recordTool(companyEquityListInputSchema, everyone, (context, input) => companyEquityList(context, input)),
	company_equity_delete: recordTool(companyEquityDeleteInputSchema, everyone, (context, input) => companyEquityDelete(context, input)),
	company_document_register: recordTool(companyDocumentRegisterInputSchema, everyone, (context, input) => companyDocumentRegister(context, input)),
	company_document_list: recordTool(companyDocumentListInputSchema, everyone, (context, input) => companyDocumentList(context, input)),
	company_document_search: recordTool(companyDocumentSearchInputSchema, everyone, (context, input) => companyDocumentSearch(context, input)),
	company_document_recall: recordTool(recallSourceInputSchema, everyone, (context, input) => companyDocumentRecall(context, input)),
	company_document_update: recordTool(companyDocumentUpdateInputSchema, everyone, (context, input) => companyDocumentUpdate(context, input)),
	company_document_upload: recordTool(companyDocumentUploadInputSchema, everyone, (context, input) => companyDocumentUpload(context, input)),
	company_document_download: recordTool(companyDocumentDownloadInputSchema, everyone, (context, input) => companyDocumentDownload(context, input)),
	circle_list: recordTool(circleListInputSchema, everyone, (context) => companyCircleList(context)),
	circle_update: recordTool(circleSetInputSchema, everyone, (context, input) => companyCircleSet(context, input)),
	circle_member_update: recordTool(circleMemberSetInputSchema, everyone, (context, input) => companyCircleMemberSet(context, input)),
	circle_channel_add: recordTool(circleChannelAddInputSchema, everyone, (context, input) => companyCircleChannelAdd(context, input)),
	circle_channel_delete: recordTool(circleChannelDeleteInputSchema, everyone, (context, input) => companyCircleChannelDelete(context, input)),
	dataroom_get: recordTool(dataRoomGetInputSchema, everyone, (context) => companyDataRoomGet(context)),
	dataroom_links_get: recordTool(dataRoomLinksGetInputSchema, everyone, (context) => companyDataRoomLinksGet(context)),
	dataroom_link_add: recordTool(dataRoomLinkInputSchema, everyone, (context, input) => companyDataRoomLinkCreate(context, input)),
	dataroom_link_delete: recordTool(dataRoomLinkRevokeSchema, everyone, (context, input) => companyDataRoomLinkRevoke(context, input)),
	dataroom_category_update: recordTool(dataRoomCategorySetInputSchema, everyone, (context, input) => companyDataRoomCategorySet(context, input)),
	dataroom_field_update: recordTool(dataRoomFieldSetInputSchema, everyone, (context, input) => companyDataRoomFieldSet(context, input)),
	dataroom_field_name_update: recordTool(dataRoomFieldNameSetInputSchema, everyone, (context, input) => companyDataRoomFieldNameSet(context, input)),
	dataroom_share_add: recordTool(dataRoomShareCreateInputSchema, everyone, (context, input) => companyDataRoomShareCreate(context, input)),
	dataroom_share_delete: recordTool(dataRoomShareRevokeInputSchema, everyone, (context, input) => companyDataRoomShareRevoke(context, input)),
	dataroom_invitation_send: recordTool(dataRoomInvitationSendInputSchema, everyone, (context, input, { place }) =>
		companyDataRoomInvitationSend(context, input, placeTheAppAnswersAt('dataroom_invitation_send', place))),
	dataroom_invitation_accept: recordTool(dataRoomInvitationAcceptInputSchema, everyone, (context, input) => companyDataRoomInvitationAccept(context, input)),
	notification_settings_get: recordTool(notificationSettingsGetInputSchema, everyone, (context) => notificationSettingsGet(context)),
	notification_settings_set: recordTool(notificationSettingsSetInputIntentSchema, everyone, (context, input) => notificationSettingsSet(context, input)),
	conversation_mute: recordTool(conversationMuteInputSchema, everyone, (context, input) => conversationMute(context, input)),
	conversation_unmute: recordTool(conversationMuteInputSchema, everyone, (context, input) => conversationUnmute(context, input)),
	channel_order_get: recordTool(channelOrderGetInputSchema, nobody, (context) => channelOrderGet(context)),
	channel_order_set: recordTool(channelOrderSetInputSchema, nobody, (context, input) => channelOrderSet(context, input)),
	channel_pin: recordTool(channelPinInputSchema, nobody, (context, input) => channelPin(context, input)),
	channel_unpin: recordTool(channelPinInputSchema, nobody, (context, input) => channelUnpin(context, input)),
	crm_organization_list: recordTool(crmOrganizationListInputSchema, nobody, (context, input) => crmOrganizationList(context, input)),
	crm_organization_add: recordTool(crmOrganizationAddInputSchema, everyone, (context, input) => crmOrganizationAdd(context, input)),
	crm_organization_update: recordTool(crmOrganizationUpdateInputSchema, everyone, (context, input) => crmOrganizationUpdate(context, input)),
	crm_organization_archive: recordTool(crmOrganizationArchiveInputSchema, everyone, (context, input) => crmOrganizationArchive(context, input), crmOrganizationArchiveTarget),
	crm_contact_list: recordTool(crmContactListInputSchema, nobody, (context, input) => crmContactList(context, input)),
	crm_contact_add: recordTool(crmContactAddInputSchema, everyone, (context, input) => crmContactAdd(context, input)),
	crm_contact_update: recordTool(crmContactUpdateInputSchema, everyone, (context, input) => crmContactUpdate(context, input)),
	crm_contact_archive: recordTool(crmContactArchiveInputSchema, everyone, (context, input) => crmContactArchive(context, input), crmContactArchiveTarget),
	crm_opportunity_list: recordTool(crmOpportunityListInputSchema, nobody, (context, input) => crmOpportunityList(context, input)),
	crm_opportunity_add: recordTool(crmOpportunityAddInputSchema, everyone, (context, input) => crmOpportunityAdd(context, input)),
	crm_opportunity_update: recordTool(crmOpportunityUpdateInputSchema, everyone, (context, input) => crmOpportunityUpdate(context, input)),
	crm_opportunity_move: recordTool(crmOpportunityMoveInputSchema, everyone, (context, input, { place }) =>
		crmOpportunityMove(context, input, exchangeRatesOf(place?.environment ?? {}))),
	crm_opportunity_archive: recordTool(crmOpportunityArchiveInputSchema, everyone, (context, input) => crmOpportunityArchive(context, input), crmOpportunityArchiveTarget),
	crm_vocabulary_get: recordTool(crmVocabularyGetInputSchema, nobody, (context) => crmVocabularyGet(context)),
	crm_vocabulary_set: recordTool(crmVocabularySetInputSchema, everyone, (context, input) => crmVocabularySet(context, input)),
	crm_record_import: recordTool(
		crmRecordImportInputSchema,
		everyone,
		async (context, input, { files }) => crmRecordImport(context, await crmImportRecordsOf(input, files)),
		(context, input, files) => crmRecordImportPreview(context, input, files)
	),
	crm_activity_list: recordTool(crmActivityListInputSchema, everyone, (context, input) => crmActivityList(context, input)),
	crm_activity_save: recordTool(crmActivitySaveInputSchema, everyone, (context, input) => crmActivitySave(context, input))
};

export function recordRunsTheTool(name: string): boolean {
	return answererOfTool(name) === 'record';
}

export function recordToolsWithoutAnImplementation(): string[] {
	return toolsTheRecordAnswers().filter((name) => !Object.hasOwn(toolsOverTheRecord, name));
}

export function toolsTheRecordRuns(): string[] {
	return Object.keys(toolsOverTheRecord);
}

export function inputSchemaTheRecordReads(name: string): z.ZodType | undefined {
	return toolsOverTheRecord[name]?.inputSchema;
}

export function toolsTheRecordAnswers(): string[] {
	return toolNamesAnsweredBy('record');
}

export type ToolAnswer = { status: number; body: unknown };

export async function previewToolOverTheRecord(
	caller: SupabaseClient,
	accountDirectory: SupabaseClient,
	requesterID: string,
	name: string,
	input: unknown,
	now: Date,
	files: CarriedWorkspaceFile[] = []
): Promise<ToolAnswer> {
	const tool = toolsOverTheRecord[name];
	if (!tool) return { status: 200, body: { tool: name, target: null } };

	try {
		const { preview, peopleRead } = tool.boundTo(inputOfTheCall(name, input));
		if (!preview) return { status: 200, body: { tool: name, target: null } };
		const context = await recordContextOf(caller, accountDirectory, requesterID, now, leavesTaskLabelsUndecided, peopleRead);
		return { status: 200, body: { tool: name, target: await preview(context, files) } };
	} catch (refusal) {
		return refusalAnswer(name, refusal);
	}
}

export async function runToolOverTheRecord(
	caller: SupabaseClient,
	accountDirectory: SupabaseClient,
	requesterID: string,
	name: string,
	input: unknown,
	now: Date,
	decideTaskLabels: TaskLabelDecider,
	files: CarriedWorkspaceFile[] = [],
	place?: ToolPlace,
	announceTaskChange?: TaskChangeAnnouncer,
	announceAttendance?: AttendanceAnnouncer
): Promise<ToolAnswer> {
	const tool = toolsOverTheRecord[name];
	if (!tool) return { status: 404, body: { error: `no tool here goes by ${name}` } };

	try {
		const { run, peopleRead } = tool.boundTo(inputOfTheCall(name, input));
		const context = () => recordContextOf(caller, accountDirectory, requesterID, now, decideTaskLabels, peopleRead);
		const result = await run({ caller, context }, { files, place, announceTaskChange, announceAttendance });
		noteWhereTheAnswerLeftItsContract(name, result);
		return { status: 200, body: { tool: name, result } };
	} catch (refusal) {
		return refusalAnswer(name, refusal);
	}
}

// The contract these tools answer under is written here, and a device reads it
// through a copy that arrives only with an OTA release, so this is the only
// side that can be fixed when an answer and the contract disagree. A device
// that refused the disagreement would report it to a machine that can do
// nothing but refuse, and would take every record tool on the fleet down until
// the next release; capabilityd therefore reads the answer by name and leaves
// the judgement here (#1486).
//
// Naming it is the whole job. The answer still goes out: the record has
// already been written by the time this runs, and losing a completed write to
// a fault no caller can fix trades one defect for a worse one. The suites hold
// the same answers to the same schema through heldToTheContract, so a shape a
// test covers fails before it ships and a shape only production reaches is
// named in the log with the field that broke.
function noteWhereTheAnswerLeftItsContract(name: string, result: unknown): void {
	const schema = capabilityToolResultSchema(name);
	if (!schema) return;
	const parsed = schema.safeParse(readWithNullAsAbsent(schema, result));
	if (parsed.success) return;
	console.error(
		`tool.answer_left_its_contract: tool=${name} ${sentencesOfSchemaRefusal(parsed.error.issues, result, 'result')}`
	);
}

function refusalAnswer(name: string, refusal: unknown): ToolAnswer {
	if (refusal instanceof Refusal) return answerOfRefusal(refusal);
	console.error('tool.failed', { tool: name, failure: refusal });
	const reason = refusal instanceof Error ? refusal.message : String(refusal);
	return { status: 500, body: { error: `${name} failed: ${reason}` } };
}
