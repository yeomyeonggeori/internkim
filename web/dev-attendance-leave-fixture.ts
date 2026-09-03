import type { EmployeeLeavePayload } from './src/routes/attendance/leave/employee-leave-types';

export function buildEmployeeLeaveFixture(): EmployeeLeavePayload {
	return {
		balanceTrackingMode: 'managed',
		leaveTypes: [
			{
				id: 'annual',
				name: '연차',
				balanceMode: 'annual',
				allowedUnits: ['fullDay', 'halfDay', 'quarterDay'],
				includeInSummary: true,
				balance: {
					usedMilliDays: 1000,
					reservedMilliDays: 500,
					availableMilliDays: 13500
				},
				isActive: true
			},
			{
				id: 'sick',
				name: '병가',
				balanceMode: 'none',
				allowedUnits: ['fullDay', 'halfDay'],
				includeInSummary: false,
				isActive: true
			},
			{
				id: 'family-event',
				name: '경조 휴가',
				balanceMode: 'separate',
				allowedUnits: ['fullDay'],
				includeInSummary: false,
				balance: {
					usedMilliDays: 0,
					reservedMilliDays: 0,
					availableMilliDays: 3000
				},
				isActive: true
			},
			{
				id: 'reward',
				name: '포상휴가',
				balanceMode: 'separate',
				allowedUnits: ['fullDay'],
				includeInSummary: true,
				balance: {
					usedMilliDays: 0,
					reservedMilliDays: 0,
					availableMilliDays: 2000
				},
				isActive: true
			}
		],
		summary: {
			usedMilliDays: 1000,
			reservedMilliDays: 500,
			availableMilliDays: 15500
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
				canCancel: true,
				createdAt: '2026-07-27T13:20:00+09:00'
			},
			{
				id: 'leave-request-needs-changes',
				leaveTypeID: 'annual',
				leaveTypeName: '연차',
				status: 'rejected',
				unit: 'quarterDay',
				startDate: '2026-08-07',
				partialPeriod: 'custom',
				startTime: '15:00',
				endTime: '17:00',
				deductionMilliDays: 250,
				reason: '관공서 방문',
				canCancel: true,
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
				canCancel: false,
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
				canCancel: true,
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
				canCancel: false,
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
				canCancel: false,
				createdAt: '2026-07-11T09:00:00+09:00',
				updatedAt: '2026-07-11T13:00:00+09:00'
			}
		],
	};
}
