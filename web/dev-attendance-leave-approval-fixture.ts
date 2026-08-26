import type { LeaveApprovalRequest } from './src/routes/attendance/approval/leave-approval-types';

export function buildTeamLeaveApprovalRequests(): LeaveApprovalRequest[] {
	return [
		{
			id: 'team-leave-request-multi-day',
			employeeEmail: 'member1@example.com',
			leaveTypeID: 'annual',
			leaveTypeName: '연차',
			balanceMode: 'annual',
			status: 'pending',
			unit: 'fullDay',
			startDate: '2026-08-24',
			endDate: '2026-08-26',
			deductionMilliDays: 3000,
			reason: '가족 여행',
			attachments: [],
			balance: { availableMilliDays: 9000, reservedMilliDays: 3000, usedMilliDays: 3000 },
			createdAt: '2026-08-18T09:12:00+09:00',
			updatedAt: '2026-08-18T09:12:00+09:00'
		},
		{
			id: 'team-leave-request-half-day',
			employeeEmail: 'park@example.com',
			leaveTypeID: 'annual',
			leaveTypeName: '연차',
			balanceMode: 'annual',
			status: 'pending',
			unit: 'halfDay',
			startDate: '2026-08-21',
			partialPeriod: 'morning',
			deductionMilliDays: 500,
			reason: '오전 병원 진료',
			attachments: [],
			balance: { availableMilliDays: 7000, reservedMilliDays: 500, usedMilliDays: 7500 },
			createdAt: '2026-08-19T18:40:00+09:00',
			updatedAt: '2026-08-19T18:40:00+09:00'
		},
		{
			id: 'team-leave-request-sick',
			employeeEmail: 'choi@example.com',
			leaveTypeID: 'sick',
			leaveTypeName: '병가',
			balanceMode: 'none',
			status: 'pending',
			unit: 'fullDay',
			startDate: '2026-08-20',
			deductionMilliDays: 1000,
			reason: '독감 진단을 받아 하루 쉬어야 합니다. 진단서를 첨부합니다.',
			attachments: [
				{
					id: 'team-leave-attachment-sick',
					fileName: 'diagnosis.pdf',
					contentType: 'application/pdf',
					sizeBytes: 128000,
					downloadURL:
						'/attendance/api/leave-requests/team-leave-request-sick/attachments/team-leave-attachment-sick'
				}
			],
			balance: { availableMilliDays: 0, reservedMilliDays: 0, usedMilliDays: 2000 },
			createdAt: '2026-08-20T07:55:00+09:00',
			updatedAt: '2026-08-20T07:55:00+09:00'
		},
		{
			id: 'team-leave-request-family-event',
			employeeEmail: 'jung@example.com',
			leaveTypeID: 'family-event',
			leaveTypeName: '경조 휴가',
			balanceMode: 'separate',
			status: 'pending',
			unit: 'fullDay',
			startDate: '2026-09-01',
			endDate: '2026-09-02',
			deductionMilliDays: 2000,
			reason: '조부상',
			attachments: [],
			balance: { availableMilliDays: 1000, reservedMilliDays: 2000, usedMilliDays: 0 },
			createdAt: '2026-08-17T21:05:00+09:00',
			updatedAt: '2026-08-17T21:05:00+09:00'
		},
		{
			id: 'team-leave-request-quarter-day',
			employeeEmail: 'kang@example.com',
			leaveTypeID: 'annual',
			leaveTypeName: '연차',
			balanceMode: 'annual',
			status: 'pending',
			unit: 'quarterDay',
			startDate: '2026-08-25',
			partialPeriod: 'custom',
			startTime: '15:30',
			endTime: '17:30',
			deductionMilliDays: 250,
			reason: '자녀 학교 상담이 있어 오후 늦게 잠깐 자리를 비우려고 합니다.',
			attachments: [],
			balance: { availableMilliDays: 12250, reservedMilliDays: 250, usedMilliDays: 2500 },
			createdAt: '2026-08-20T11:30:00+09:00',
			updatedAt: '2026-08-20T11:30:00+09:00'
		}
	];
}
