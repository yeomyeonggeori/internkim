import type { EmployeeLeavePayload } from './src/routes/attendance/leave/employee-leave-types';

export function buildEmployeeLeaveFixture(): EmployeeLeavePayload {
	return {
		leaveTypes: [
			{
				id: 'annual',
				name: '연차',
				balanceMode: 'annual',
				allowedUnits: ['fullDay', 'halfDay', 'quarterDay'],
				isActive: true
			},
			{
				id: 'sick',
				name: '병가',
				balanceMode: 'none',
				allowedUnits: ['fullDay', 'halfDay'],
				isActive: true
			},
			{
				id: 'family-event',
				name: '경조 휴가',
				balanceMode: 'separate',
				allowedUnits: ['fullDay'],
				isActive: true
			}
		],
		summary: {
			usedMilliDays: 1000,
			reservedMilliDays: 500,
			availableMilliDays: 13500
		},
		requests: [
			{
				id: 'leave-request-pending',
				leaveTypeID: 'annual',
				leaveTypeName: '연차',
				status: 'pending',
				unit: 'quarterDay',
				startDate: '2026-08-03',
				partialPeriod: 'custom',
				startTime: '14:00',
				endTime: '16:00',
				deductionMilliDays: 250,
				reason: '개인 일정',
				attachments: [
					{
						id: 'leave-attachment-pending',
						fileName: 'personal-schedule.pdf',
						contentType: 'application/pdf',
						sizeBytes: 96000,
						downloadURL:
							'/attendance/api/leave-requests/leave-request-pending/attachments/leave-attachment-pending'
					}
				],
				canCancel: true,
				canEdit: true,
				canResubmit: false,
				revision: 1,
				createdAt: '2026-07-27T13:20:00+09:00'
			},
			{
				id: 'leave-request-needs-changes',
				leaveTypeID: 'annual',
				leaveTypeName: '연차',
				status: 'needsChanges',
				unit: 'quarterDay',
				startDate: '2026-08-07',
				partialPeriod: 'custom',
				startTime: '15:00',
				endTime: '17:00',
				deductionMilliDays: 250,
				reason: '관공서 방문',
				adminResponse: '방문 일정을 확인할 수 있는 자료를 보완해 주세요.',
				attachments: [
					{
						id: 'leave-attachment-existing',
						fileName: 'appointment.pdf',
						contentType: 'application/pdf',
						sizeBytes: 184000,
						downloadURL:
							'/attendance/api/leave-requests/leave-request-needs-changes/attachments/leave-attachment-existing'
					}
				],
				canCancel: true,
				canEdit: false,
				canResubmit: true,
				revision: 2,
				createdAt: '2026-07-25T10:00:00+09:00',
				updatedAt: '2026-07-26T09:30:00+09:00'
			},
			{
				id: 'leave-request-approved',
				leaveTypeID: 'annual',
				leaveTypeName: '연차',
				status: 'approved',
				unit: 'halfDay',
				startDate: '2026-07-21',
				partialPeriod: 'afternoon',
				startTime: '14:00',
				endTime: '18:00',
				deductionMilliDays: 500,
				reason: '가족 일정',
				attachments: [],
				canCancel: false,
				canEdit: false,
				canResubmit: false,
				revision: 2,
				createdAt: '2026-07-17T16:10:00+09:00',
				updatedAt: '2026-07-18T11:00:00+09:00'
			},
			{
				id: 'leave-request-approved-morning',
				leaveTypeID: 'annual',
				leaveTypeName: '연차',
				status: 'approved',
				unit: 'halfDay',
				startDate: '2026-08-10',
				partialPeriod: 'morning',
				startTime: '09:00',
				endTime: '13:00',
				deductionMilliDays: 500,
				reason: '개인 용무',
				attachments: [],
				canCancel: true,
				canEdit: false,
				canResubmit: false,
				revision: 1,
				createdAt: '2026-07-03T15:20:00+09:00',
				updatedAt: '2026-07-04T10:10:00+09:00'
			},
			{
				id: 'leave-request-cancelled',
				leaveTypeID: 'annual',
				leaveTypeName: '연차',
				status: 'cancelled',
				unit: 'halfDay',
				startDate: '2026-07-15',
				partialPeriod: 'morning',
				startTime: '09:00',
				endTime: '13:00',
				deductionMilliDays: 500,
				reason: '일정 취소',
				attachments: [],
				canCancel: false,
				canEdit: false,
				canResubmit: false,
				revision: 3,
				createdAt: '2026-07-09T09:20:00+09:00',
				updatedAt: '2026-07-11T11:30:00+09:00'
			},
			{
				id: 'leave-request-rejected',
				leaveTypeID: 'sick',
				leaveTypeName: '병가',
				status: 'rejected',
				unit: 'fullDay',
				startDate: '2026-07-10',
				endDate: '2026-07-10',
				deductionMilliDays: 0,
				reason: '병원 진료',
				adminResponse: '요청한 날짜가 이미 지난 뒤 제출되었습니다.',
				attachments: [],
				canCancel: false,
				canEdit: false,
				canResubmit: false,
				revision: 2,
				createdAt: '2026-07-11T09:00:00+09:00',
				updatedAt: '2026-07-11T13:00:00+09:00'
			}
		],
		ledgerEntries: [
			{
				id: 'leave-ledger-reserve-quarter',
				operationKey: 'reserve:leave-request-needs-changes',
				operationType: 'reserve',
				occurredAt: '2026-07-25T10:00:00+09:00',
				leaveTypeID: 'annual',
				leaveTypeName: '연차',
				deltaMilliDays: -250,
				balanceAfterMilliDays: 13750,
				isUntracked: false,
				requestID: 'leave-request-needs-changes'
			},
			{
				id: 'leave-ledger-reserve-quarter-pending',
				operationKey: 'reserve:leave-request-pending',
				operationType: 'reserve',
				occurredAt: '2026-07-27T13:20:00+09:00',
				leaveTypeID: 'annual',
				leaveTypeName: '연차',
				deltaMilliDays: -250,
				balanceAfterMilliDays: 13500,
				isUntracked: false,
				requestID: 'leave-request-pending'
			},
			{
				id: 'leave-ledger-use',
				operationKey: 'use:2026-07-21',
				operationType: 'use',
				occurredAt: '2026-07-21T18:00:00+09:00',
				leaveTypeID: 'annual',
				leaveTypeName: '연차',
				deltaMilliDays: -500,
				balanceAfterMilliDays: 14000,
				isUntracked: false,
				requestID: 'leave-request-approved'
			},
			{
				id: 'leave-ledger-use-morning',
				operationKey: 'use:2026-08-10',
				operationType: 'use',
				occurredAt: '2026-07-08T13:00:00+09:00',
				leaveTypeID: 'annual',
				leaveTypeName: '연차',
				deltaMilliDays: -500,
				balanceAfterMilliDays: 14500,
				isUntracked: false,
				requestID: 'leave-request-approved-morning'
			},
			{
				id: 'leave-ledger-restore-cancelled',
				operationKey: 'restore:leave-request-cancelled',
				operationType: 'restore',
				occurredAt: '2026-07-11T11:30:00+09:00',
				leaveTypeID: 'annual',
				leaveTypeName: '연차',
				deltaMilliDays: 500,
				balanceAfterMilliDays: 14500,
				isUntracked: false,
				requestID: 'leave-request-cancelled'
			},
			{
				id: 'leave-ledger-use-cancelled',
				operationKey: 'use:leave-request-cancelled',
				operationType: 'use',
				occurredAt: '2026-07-10T10:00:00+09:00',
				leaveTypeID: 'annual',
				leaveTypeName: '연차',
				deltaMilliDays: -500,
				balanceAfterMilliDays: 14000,
				isUntracked: false,
				requestID: 'leave-request-cancelled'
			},
			{
				id: 'leave-ledger-grant',
				operationKey: 'grant:2026',
				operationType: 'grant',
				occurredAt: '2026-01-01T00:00:00+09:00',
				leaveTypeID: 'annual',
				leaveTypeName: '연차',
				deltaMilliDays: 15000,
				balanceAfterMilliDays: 15000,
				isUntracked: false
			}
		]
	};
}
