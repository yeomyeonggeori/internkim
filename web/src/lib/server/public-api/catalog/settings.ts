import { z } from 'zod';

import {
  CapabilityAnsweredBy,
  CapabilityEstimatedLatency,
  CapabilitySideEffect,
} from './protocol';

import type { CapabilityToolDefinition } from './definition';

const resourceIDSchema = z.string()
  .min(1)
  .regex(/^\S(?:.*\S)?$/, 'Resource identity must not have leading or trailing whitespace.');

const dayDescription = 'A day as yyyy-mm-dd.';
const clockDescription = 'A time of day as 24-hour HH:MM.';

export enum WorkspaceAttendanceWorkMode {
  Autonomous = 'autonomous',
  Flexible = 'flexible',
  Fixed = 'fixed',
}

export enum WorkspaceLeaveBalanceTracking {
  Managed = 'managed',
  Unlimited = 'unlimited',
}

export enum WorkspaceLeaveBalanceMode {
  Annual = 'annual',
  Separate = 'separate',
  None = 'none',
}

export enum WorkspaceLeaveGrantCadence {
  Annual = 'annual',
  Monthly = 'monthly',
  None = 'none',
}

export enum WorkspaceLeaveExpiryMode {
  FiscalYearEnd = 'fiscalYearEnd',
  MonthsAfterGrant = 'monthsAfterGrant',
  None = 'none',
}

export enum WorkspaceLeaveUnit {
  FullDay = 'fullDay',
  HalfDay = 'halfDay',
  QuarterDay = 'quarterDay',
}

const workLocationSchema = z.strictObject({
  name: z.string().min(1).max(64).describe("The workplace's name as people say it, e.g. 'the office' or 'home'."),
  color: z.string().max(32).describe('The colour the attendance screens draw it in, as a CSS colour. Omit to leave it uncoloured.').optional(),
});

export const companySettingsGetInputSchema = z.strictObject({});

export const companySettingsUpdateInputSchema = z.strictObject({
  name: z.string().min(1).max(256).describe("The company's name as the app shows it. Not the legal name a document prints, which is company_info_set's.").optional(),
  locale: z.string().min(2).max(16).describe("The language the company works in, as a BCP 47 tag, e.g. 'ko' or 'en-US'.").optional(),
  timeZone: z.string().min(1).max(64).describe("The company's time zone as an IANA name, e.g. 'Asia/Seoul'. Every date and time the record answers is read in it.").optional(),
  currencyCode: z.string().length(3).describe("The currency amounts are held in, as an ISO 4217 code, e.g. 'KRW'.").optional(),
  workLocations: z.array(workLocationSchema).describe('The workplaces attendance can be recorded at, in the order the screens list them. Replaces the whole list.').optional(),
  leaveDays: z.number().describe("The annual leave a person is granted, in days. It is the leave policy's annual grant, so this writes that, and attendance_leave_policy_set is where the rest of the policy is written.").optional(),
  teamViewVisibleToAll: z.boolean().describe("Whether everybody sees the whole company's attendance, or only the administrators do.").optional(),
});

export const companySettingsUpdateInputIntentSchema = companySettingsUpdateInputSchema.partial();

export const companySettingsResultSchema = z.strictObject({
  name: z.string(),
  country: z.string(),
  locale: z.string(),
  timeZone: z.string(),
  currencyCode: z.string(),
  workLocations: z.array(z.strictObject({ name: z.string(), color: z.string().nullable() })),
  leaveDays: z.number().nullable(),
  teamViewVisibleToAll: z.boolean(),
  profileImageURL: z.string().nullable(),
});

const holidayHintSchema = z.string().min(1).max(256).describe(
  'Identifies the company holiday: its exact holiday ID, its exact date as yyyy-mm-dd, or its name as a company_holiday_list result shows it. A name only one holiday carries resolves; a name several carry fails with the candidates to choose between.',
);

export const companyHolidayListInputSchema = z.strictObject({
  year: z.number().int().describe('Only the holidays falling in this calendar year. A holiday that recurs annually is counted in every year. Omit for all of them.').optional(),
});

export const companyHolidayAddInputSchema = z.strictObject({
  date: z.string().describe(`The day the company is closed. ${dayDescription}`),
  name: z.string().min(1).max(120).describe("What the day is called, e.g. 'National Foundation Day'."),
  recursAnnually: z.boolean().describe('Set true for a day that falls on the same date every year. Defaults to false, which is what a one-off closure is.').optional(),
});

export const companyHolidayAddInputIntentSchema = companyHolidayAddInputSchema.partial();

export const companyHolidayUpdateInputSchema = z.strictObject({
  holidayHint: holidayHintSchema,
  date: z.string().describe(`The day it actually falls on. ${dayDescription} Omit to keep the day it has.`).optional(),
  name: z.string().min(1).max(120).describe('What the day should be called. Omit to keep the name it has.').optional(),
  recursAnnually: z.boolean().describe('Whether it falls on the same date every year. Omit to keep what it says now.').optional(),
});

export const companyHolidayUpdateInputIntentSchema = companyHolidayUpdateInputSchema.omit({ holidayHint: true });

export const companyHolidayDeleteInputSchema = z.strictObject({
  holidayHint: holidayHintSchema,
});

export const companyHolidayDeleteInputIntentSchema = z.strictObject({});

export const companyHolidayResultSchema = z.strictObject({
  holidayID: resourceIDSchema,
  name: z.string(),
  date: z.string(),
  recursAnnually: z.boolean(),
  createdAt: z.string().nullable(),
  updatedAt: z.string().nullable(),
});

export const companyHolidayListResultSchema = z.strictObject({
  count: z.number().int(),
  year: z.number().int().nullable(),
  holidays: z.array(companyHolidayResultSchema),
});

const breakPeriodSchema = z.strictObject({
  startTime: z.string().describe(`When the break starts. ${clockDescription}`),
  endTime: z.string().describe(`When the break ends. ${clockDescription}`),
});

const workPolicyFields = {
  workMode: z.enum(WorkspaceAttendanceWorkMode).describe('fixed for set hours everybody keeps, flexible for hours a person chooses around a core, autonomous for no hours at all.'),
  workingWeekdays: z.array(z.number().int()).describe('The weekdays that are working days, Monday as 1 through Sunday as 7.'),
  dailyTargetMinutes: z.number().int().describe('The minutes a working day is expected to hold, breaks excluded. Zero only under autonomous work.'),
  weeklyTargetMinutes: z.number().int().describe('The minutes a working week is expected to hold: dailyTargetMinutes times the working weekdays, and zero under autonomous work.'),
  referenceStartTime: z.string().describe(`The hour a day is measured from when nobody is held to fixed hours. ${clockDescription}`),
  fixedStartTime: z.string().describe(`When the day starts under fixed work. ${clockDescription} An empty string under any other work mode.`),
  fixedEndTime: z.string().describe(`When the day ends under fixed work. ${clockDescription} An empty string under any other work mode.`),
  coreTimeEnabled: z.boolean().describe('Whether flexible work has hours everybody has to be present for. False under fixed and autonomous work.'),
  coreStartTime: z.string().describe(`When the core hours start. ${clockDescription} An empty string when there are none.`),
  coreEndTime: z.string().describe(`When the core hours end. ${clockDescription} An empty string when there are none.`),
  breakPeriods: z.array(breakPeriodSchema).describe('The breaks the day holds, which do not count toward the target. They may not overlap.'),
  nightStartTime: z.string().describe(`When night work starts. ${clockDescription}`),
  nightEndTime: z.string().describe(`When night work ends. ${clockDescription}`),
};

export const attendanceWorkPolicyGetInputSchema = z.strictObject({});

export const attendanceWorkPolicySetInputSchema = z.strictObject(workPolicyFields);

export const attendanceWorkPolicySetInputIntentSchema = attendanceWorkPolicySetInputSchema.partial();

export const attendanceWorkPolicyRevisionResultSchema = z.strictObject({
  effectiveDate: z.string(),
  workMode: z.string(),
  workingWeekdays: z.array(z.number().int()),
  dailyTargetMinutes: z.number().int(),
  weeklyTargetMinutes: z.number().int(),
  referenceStartTime: z.string(),
  fixedStartTime: z.string(),
  fixedEndTime: z.string(),
  coreTimeEnabled: z.boolean(),
  coreStartTime: z.string(),
  coreEndTime: z.string(),
  breakPeriods: z.array(z.strictObject({ startTime: z.string(), endTime: z.string() })),
  nightStartTime: z.string(),
  nightEndTime: z.string(),
});

const attendanceWorkPolicyStoredSchema = z.strictObject({
  version: z.number().int(),
  revisions: z.array(attendanceWorkPolicyRevisionResultSchema),
});

export const attendanceWorkPolicyResultSchema = z.strictObject({
  timeZone: z.string(),
  workMode: z.string(),
  policy: attendanceWorkPolicyStoredSchema.nullable(),
  people: z.array(z.strictObject({
    personID: resourceIDSchema,
    workHours: z.array(z.array(z.unknown())).nullable(),
    minimumDailyMinutes: z.number().int().nullable(),
  })),
});

export const attendanceWorkPolicySetResultSchema = z.strictObject({
  timeZone: z.string(),
  effectiveDate: z.string(),
  policy: attendanceWorkPolicyStoredSchema,
});

const leaveTypeFields = {
  id: z.string().max(64).describe("The leave type's stable id. An empty string on a type being added, which the record then names."),
  systemKind: z.string().max(64).describe('The kind a built-in type is, e.g. annual or sick. An empty string on a type this company invented.'),
  name: z.string().min(1).max(120).describe("What the type is called, e.g. 'annual leave'."),
  paid: z.boolean().describe('Whether leave of this type is paid.'),
  balanceMode: z.enum(WorkspaceLeaveBalanceMode).describe('annual for the type that owns the annual balance and the types that draw on it, separate for a type counting its own, none for a type counting nothing.'),
  grantCadence: z.enum(WorkspaceLeaveGrantCadence).describe('How often a balance is granted. none for a type that grants nothing.'),
  grantAmountMilliDays: z.number().int().describe('How much is granted each time, in thousandths of a day. Zero for a type that grants nothing.'),
  expiryMode: z.enum(WorkspaceLeaveExpiryMode).describe('When an unused balance lapses.'),
  expiryMonths: z.number().int().describe('How many months after the grant it lapses. Only under monthsAfterGrant.').optional(),
  carryoverEnabled: z.boolean().describe('Whether an unused balance carries into the next year.'),
  carryoverLimitMilliDays: z.number().int().describe('The most that carries over, in thousandths of a day. Omit for no limit.').optional(),
  usageLimitMilliDays: z.number().int().describe('The most a person may take of this type in a year, in thousandths of a day. Omit for no ceiling.').optional(),
  allowedUnits: z.array(z.enum(WorkspaceLeaveUnit)).describe('The portions of a day this type may be taken in. At least one, each named once.'),
  includeInSummary: z.boolean().describe('Whether the type appears in the leave summary a person sees.'),
  isActive: z.boolean().describe('Whether the type may still be chosen. An inactive type keeps the leave already taken under it.'),
  isSystem: z.boolean().describe('Whether the type is one the product ships. Only a type whose id names a built-in kind may say true.'),
  sortOrder: z.number().int().describe('Where the type sits in the list, counting from 0.'),
};

const leavePolicyFields = {
  balanceTrackingMode: z.enum(WorkspaceLeaveBalanceTracking).describe('managed when the company grants a fixed annual entitlement, unlimited when it grants none.'),
  fiscalYearStartMonth: z.number().int().describe('The month the leave year starts in, 1 through 12.'),
  fiscalYearStartDay: z.number().int().describe('The day of that month the leave year starts on. It has to be a real date, so the 29th of February is refused.'),
  leaveTypes: z.array(z.strictObject(leaveTypeFields)).describe('Every leave type this company offers, replacing the whole list. A type left out that somebody has already taken leave under is kept, deactivated.'),
};

export const attendanceLeavePolicyGetInputSchema = z.strictObject({});

export const attendanceLeavePolicySetInputSchema = z.strictObject(leavePolicyFields);

export const attendanceLeavePolicySetInputIntentSchema = attendanceLeavePolicySetInputSchema.partial();

export const attendanceLeavePolicyResultSchema = z.strictObject({
  version: z.number().int(),
  balanceTrackingMode: z.string(),
  fiscalYearStartMonth: z.number().int(),
  fiscalYearStartDay: z.number().int(),
  leaveTypes: z.array(z.strictObject({
    id: z.string(),
    systemKind: z.string(),
    name: z.string(),
    paid: z.boolean(),
    balanceMode: z.string(),
    grantCadence: z.string(),
    grantAmountMilliDays: z.number().int(),
    expiryMode: z.string(),
    expiryMonths: z.number().int().optional(),
    carryoverEnabled: z.boolean(),
    carryoverLimitMilliDays: z.number().int().optional(),
    usageLimitMilliDays: z.number().int().optional(),
    allowedUnits: z.array(z.string()),
    includeInSummary: z.boolean(),
    isActive: z.boolean(),
    isSystem: z.boolean(),
    sortOrder: z.number().int(),
  })),
  updatedAt: z.string(),
});

export const settingsToolDefinitions: CapabilityToolDefinition[] = [
  {
    name: 'company_settings_get',
    namespace: 'company',
    answeredBy: CapabilityAnsweredBy.Record,
    privacyClass: 'workspace_company',
    policyResource: 'tool:company_settings_get',
    description: "What the company is set to: its name, country, language, time zone, base currency, the workplaces attendance can be recorded at, the annual leave a person is granted, whether everybody may see the whole company's attendance, and the address of its picture. Read this to answer 'what time zone are we on', 'which currency do we use', or 'where can we clock in from'. The workplace names are what attendance_add's location takes.",
    version: '2',
    estimatedLatency: CapabilityEstimatedLatency.Low,
    inputSchema: companySettingsGetInputSchema,
    result: { schema: companySettingsResultSchema, effects: [] },
    sideEffect: CapabilitySideEffect.Read,
  },
  {
    name: 'company_settings_update',
    namespace: 'company',
    answeredBy: CapabilityAnsweredBy.Record,
    privacyClass: 'workspace_company',
    policyResource: 'tool:company_settings_update',
    description: 'Change what the company is set to. Only the fields the call names change. This is an administrator\'s, and it is company-wide: the time zone and the language decide what every colleague sees, so put it to the requester before calling.',
    version: '2',
    estimatedLatency: CapabilityEstimatedLatency.Medium,
    inputSchema: companySettingsUpdateInputSchema,
    inputIntentSchema: companySettingsUpdateInputIntentSchema,
    result: { schema: companySettingsResultSchema, effects: [] },
    sideEffect: CapabilitySideEffect.WorkspaceWrite,
    requiresApproval: true,
    completionEvidence: { mode: 'success', action: 'write_company', targetKind: 'company' },
  },
  {
    name: 'company_holiday_list',
    namespace: 'company',
    answeredBy: CapabilityAnsweredBy.Record,
    privacyClass: 'workspace_company',
    policyResource: 'tool:company_holiday_list',
    description: "The days this company is closed, earliest first. Use this to answer 'are we closed on the third of October' or to find the holiday another company_holiday tool is about to change. A holiday that recurs annually falls on the same date every year, so it is answered for whichever year is asked for.",
    version: '1',
    estimatedLatency: CapabilityEstimatedLatency.Low,
    inputSchema: companyHolidayListInputSchema,
    result: { schema: companyHolidayListResultSchema, effects: [] },
    sideEffect: CapabilitySideEffect.Read,
  },
  {
    name: 'company_holiday_add',
    namespace: 'company',
    answeredBy: CapabilityAnsweredBy.Record,
    privacyClass: 'workspace_company',
    policyResource: 'tool:company_holiday_add',
    description: 'Add a day the company is closed. Adding a holiday is an administrator\'s. A date the company already holds a holiday on is refused, so read company_holiday_list first when the requester may be repeating one.',
    version: '1',
    estimatedLatency: CapabilityEstimatedLatency.Medium,
    inputSchema: companyHolidayAddInputSchema,
    inputIntentSchema: companyHolidayAddInputIntentSchema,
    result: { schema: companyHolidayResultSchema, effects: [] },
    sideEffect: CapabilitySideEffect.WorkspaceWrite,
    completionEvidence: { mode: 'success', action: 'write_company', targetKind: 'company' },
  },
  {
    name: 'company_holiday_update',
    namespace: 'company',
    answeredBy: CapabilityAnsweredBy.Record,
    privacyClass: 'workspace_company',
    policyResource: 'tool:company_holiday_update',
    description: 'Change the day, the name, or the yearly recurrence of a holiday the company already holds. Only the fields the call names change. Changing a holiday is an administrator\'s.',
    version: '1',
    estimatedLatency: CapabilityEstimatedLatency.Medium,
    inputSchema: companyHolidayUpdateInputSchema,
    inputIntentSchema: companyHolidayUpdateInputIntentSchema,
    result: { schema: companyHolidayResultSchema, effects: [] },
    sideEffect: CapabilitySideEffect.WorkspaceWrite,
    completionEvidence: { mode: 'success', action: 'write_company', targetKind: 'company' },
  },
  {
    name: 'company_holiday_delete',
    namespace: 'company',
    answeredBy: CapabilityAnsweredBy.Record,
    privacyClass: 'workspace_company',
    policyResource: 'tool:company_holiday_delete',
    description: 'Take a day off the company holiday list, so it counts as a working day again. Removing a holiday is an administrator\'s, and it changes what every colleague is expected to work, so put it to the requester before calling.',
    version: '1',
    estimatedLatency: CapabilityEstimatedLatency.Medium,
    inputSchema: companyHolidayDeleteInputSchema,
    inputIntentSchema: companyHolidayDeleteInputIntentSchema,
    result: { schema: companyHolidayResultSchema, effects: [] },
    sideEffect: CapabilitySideEffect.Destructive,
    requiresApproval: true,
  },
  {
    name: 'attendance_work_policy_get',
    namespace: 'attendance',
    answeredBy: CapabilityAnsweredBy.Record,
    privacyClass: 'workspace_attendance',
    policyResource: 'tool:attendance_work_policy_get',
    description: "The hours this company works: the policy in force and every earlier one with the day it took effect, so a past day is judged by the policy of that day. Use this to answer 'what are our working hours', 'is Saturday a working day', or 'when did the four-day week start'. people carries the work hours and daily minimum held for one colleague where they differ from the company's.",
    version: '1',
    estimatedLatency: CapabilityEstimatedLatency.Low,
    inputSchema: attendanceWorkPolicyGetInputSchema,
    result: { schema: attendanceWorkPolicyResultSchema, effects: [] },
    sideEffect: CapabilitySideEffect.Read,
  },
  {
    name: 'attendance_work_policy_set',
    namespace: 'attendance',
    answeredBy: CapabilityAnsweredBy.Record,
    privacyClass: 'workspace_attendance',
    policyResource: 'tool:attendance_work_policy_set',
    description: "Set the hours this company works from today on. The earlier policies stay, each with the day it took effect, so nothing already worked is re-judged. The whole policy is written at once, so read attendance_work_policy_get first and send it back with the fields that change. This is an administrator's and it is company-wide.",
    version: '1',
    estimatedLatency: CapabilityEstimatedLatency.Medium,
    inputSchema: attendanceWorkPolicySetInputSchema,
    inputIntentSchema: attendanceWorkPolicySetInputIntentSchema,
    result: { schema: attendanceWorkPolicySetResultSchema, effects: [] },
    sideEffect: CapabilitySideEffect.WorkspaceWrite,
    requiresApproval: true,
    completionEvidence: { mode: 'success', action: 'write_company', targetKind: 'company' },
  },
  {
    name: 'attendance_leave_policy_get',
    namespace: 'attendance',
    answeredBy: CapabilityAnsweredBy.Record,
    privacyClass: 'workspace_leave',
    policyResource: 'tool:attendance_leave_policy_get',
    description: "The leave this company offers: every type with whether it is paid, how much it grants and in what portions of a day it may be taken, and the day the leave year starts. Use this to answer 'what leave can I take' or 'is sick leave paid'. The type names are what leave_request's kind takes.",
    version: '1',
    estimatedLatency: CapabilityEstimatedLatency.Low,
    inputSchema: attendanceLeavePolicyGetInputSchema,
    result: { schema: attendanceLeavePolicyResultSchema, effects: [] },
    sideEffect: CapabilitySideEffect.Read,
  },
  {
    name: 'attendance_leave_policy_set',
    namespace: 'attendance',
    answeredBy: CapabilityAnsweredBy.Record,
    privacyClass: 'workspace_leave',
    policyResource: 'tool:attendance_leave_policy_set',
    description: "Set the leave this company offers. The whole policy is written at once, so read attendance_leave_policy_get first and send it back with the types that change. A type left out that somebody has already taken leave under is kept, deactivated, because the leave rows point at it. This is an administrator's and it is company-wide.",
    version: '1',
    estimatedLatency: CapabilityEstimatedLatency.Medium,
    inputSchema: attendanceLeavePolicySetInputSchema,
    inputIntentSchema: attendanceLeavePolicySetInputIntentSchema,
    result: { schema: attendanceLeavePolicyResultSchema, effects: [] },
    sideEffect: CapabilitySideEffect.WorkspaceWrite,
    requiresApproval: true,
    completionEvidence: { mode: 'success', action: 'write_company', targetKind: 'company' },
  },
];

export type CompanySettingsUpdateInput = z.infer<typeof companySettingsUpdateInputSchema>;
export type CompanySettingsResult = z.infer<typeof companySettingsResultSchema>;
export type CompanyHolidayListInput = z.infer<typeof companyHolidayListInputSchema>;
export type CompanyHolidayAddInput = z.infer<typeof companyHolidayAddInputSchema>;
export type CompanyHolidayUpdateInput = z.infer<typeof companyHolidayUpdateInputSchema>;
export type CompanyHolidayDeleteInput = z.infer<typeof companyHolidayDeleteInputSchema>;
export type CompanyHolidayResult = z.infer<typeof companyHolidayResultSchema>;
export type CompanyHolidayListResult = z.infer<typeof companyHolidayListResultSchema>;
export type AttendanceWorkPolicySetInput = z.infer<typeof attendanceWorkPolicySetInputSchema>;
export type AttendanceWorkPolicyResult = z.infer<typeof attendanceWorkPolicyResultSchema>;
export type AttendanceWorkPolicySetResult = z.infer<typeof attendanceWorkPolicySetResultSchema>;
export type AttendanceLeavePolicySetInput = z.infer<typeof attendanceLeavePolicySetInputSchema>;
export type AttendanceLeavePolicyResult = z.infer<typeof attendanceLeavePolicyResultSchema>;
