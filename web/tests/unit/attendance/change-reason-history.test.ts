import {describe, expect, test} from 'bun:test';
import {attendanceChangeReasonOf, attendanceChangeReasonLabel, attendanceChangeReasonText} from '../../../src/lib/attendance/change-reason';
import {canUndoHandWrittenRecord, type HandWrittenRecord} from '../../../src/routes/attendance/hand-written/hand-written-records';
describe('change reason and historical undo provenance', () => {
 test('new enum codes map to the same canonical labels in storage and table', () => {
  for(const code of ['record_missing','time_correction','location_correction','other'] as const) expect(attendanceChangeReasonLabel(attendanceChangeReasonText(code),'ko')).toBe(attendanceChangeReasonLabel(code,'ko'));
  expect(attendanceChangeReasonLabel('location_correction','en')).toBe('Work location correction');
 });
 test('unknown legacy free text displays Other while its stored value remains untouched', () => {
  const raw = 'private old reason'; expect(attendanceChangeReasonOf(raw)).toBe('other'); expect(attendanceChangeReasonLabel(raw,'ko')).toBe('기타'); expect(raw).toBe('private old reason');
  expect(attendanceChangeReasonLabel('시각 정정','ko')).toBe('시간 정정'); expect(attendanceChangeReasonLabel('누락된 기록 추가','ko')).toBe('기록 누락');
 });
 test('unknown legacy additions cannot be inferred as deletions; actual captured additions and corrections can undo', () => {
  const row: HandWrittenRecord = {eventID:'e',person:'P',kind:'clock_in',date:'2026-10-05',time:'08:30',originalDate:null,originalTime:null,reason:'기타',changedBySource:'legacy_subject'};
  expect(canUndoHandWrittenRecord(row)).toBe(false);
  expect(canUndoHandWrittenRecord({...row,changedBySource:'observed'})).toBe(true);
  expect(canUndoHandWrittenRecord({...row,originalDate:'2026-10-05',originalTime:'08:30',previousRecorded:true,originalLocation:'외근'})).toBe(true);
 });
});
