import { organizationText } from './organization-text';

export const adminText = {
	ko: {
		messages: {
			userSaveError: '사용자 저장에 실패했습니다.'
		},
		companyHolidays: {
			title: '회사 지정 휴일', description: '창립기념일이나 전사 휴무일을 등록하면 모든 구성원의 캘린더와 휴가 계산에 반영됩니다.', add: '휴일 추가', createTitle: '회사 휴일 등록', editTitle: '회사 휴일 수정', name: '휴일명', namePlaceholder: '예: 창립기념일', date: '날짜', recursAnnually: '매년 반복', recurrenceDescription: '선택한 월과 일에 매년 표시합니다. 주말과 겹쳐도 대체 휴일은 자동으로 만들지 않습니다.', annual: '매년 반복', oneTime: '1회', edit: '수정', remove: '삭제', cancel: '취소', save: '저장', loading: '불러오는 중...', empty: '등록된 회사 휴일이 없습니다.', loadError: '회사 휴일을 불러오지 못했습니다.', saveError: '회사 휴일을 저장하지 못했습니다.', saveSuccess: '회사 휴일을 저장했습니다.', removeError: '회사 휴일을 삭제하지 못했습니다.', removeSuccess: '회사 휴일을 삭제했습니다.', removeConfirmationTitle: '회사 휴일을 삭제할까요?', removeConfirmationDescription: '삭제하면 모든 구성원의 캘린더와 휴가 계산에서 더 이상 적용되지 않습니다.'
		},
		workSettings: {
			title: '근무 설정',
			description: '회사의 근무 방식과 근무시간 계산 기준을 관리합니다.',
			workMode: '근무 방식',
			workModeDescription: '회사에서 사용하는 근무 방식 하나를 선택합니다.',
			autonomous: '자율 근무제',
			autonomousDescription: '기준시간 없이 실제 근무시간만 기록합니다.',
			flexible: '유연 근무제',
			flexibleDescription: '주 기준시간과 코어타임 안에서 자유롭게 근무합니다.',
			fixed: '고정 근무제',
			fixedDescription: '정해진 출퇴근 시간과 근무 요일을 적용합니다.',
			workingWeekdays: '근무 요일',
			weekdays: ['월', '화', '수', '목', '금', '토', '일'],
			hourUnit: '시간',
			minuteUnit: '분',
			start: '시작',
			end: '종료',
			weeklyTarget: '주 기준 근무시간',
			referenceStart: '휴가 기준 시작시간',
			coreTime: '코어타임',
			coreTimeEnabled: '코어타임 사용',
			fixedHours: '출퇴근 시간',
			commonRules: '공통 기준',
			commonRulesDescription: '실제 근무시간과 야간근무 계산에 적용됩니다.',
			breakPeriods: '휴게시간',
			addBreak: '휴게 구간 추가',
			nightHours: '야간근무 시간대',
			remove: '제거',
			preview: '직원 화면 기준 미리보기',
			previewDescription: '내 근무 시간 Bar와 직원 근무 현황에 적용됩니다.',
			dailyTarget: '일별 기준',
			weeklyTargetPreview: '주별 기준',
			monthlyTarget: '이번 달 기준',
			noBaseline: '기준시간 없음',
			autonomousNotice: '직원에게 기준선 없이 실제 근무시간과 야간근무만 보여줍니다.',
			save: '변경사항 저장',
			loading: '근무 설정을 불러오는 중...',
			loadError: '근무 설정을 불러오지 못했습니다.',
			saveError: '근무 설정을 저장하지 못했습니다.',
			saveSuccess: '근무 설정을 저장했습니다.',
			invalidWeekdays: '근무 요일을 하나 이상 선택하세요.',
			invalidTarget: '기준 근무시간을 다시 확인하세요.'
		},
		attendanceSettings: {
			title: '휴가 설정', description: '기본 휴가와 회사 사용자 정의 휴가 종류를 관리합니다.', systemBadge: '기본 휴가', add: '휴가 종류 추가', newLeave: '새 휴가', unsaved: '저장 전', inactive: '사용 중지', paid: '유급', paidDescription: '이 휴가를 유급으로 처리할지 설정합니다.', unpaid: '무급', balanceMode: '차감 방식', allowedUnits: '신청 단위', allowedUnitsDescription: '더 작은 단위를 켜면 그보다 큰 단위도 함께 켜집니다.', fullDay: '종일', halfDay: '반차', quarterDay: '반반차', dayUnit: '일', increaseDays: '하루 늘리기', decreaseDays: '하루 줄이기', name: '이름', namePlaceholder: '예: 가족돌봄 휴가', systemKind: '시스템 종류', balanceModeAnnual: '연차 차감', balanceModeSeparate: '별도 잔액', balanceModeNone: '차감 없음', balanceModeUnavailable: '이 회사에서는 쓸 수 없음', includeInSummary: '휴가 현황 합계에 포함', includeInSummaryDescription: '직원 근태 화면의 사용·승인 대기·남음 합계에 포함합니다.', annualBalanceDescription: '이 휴가는 별도 잔액을 만들지 않고 연차 잔액에서 차감합니다.', grantCadence: '부여 주기', grantCadenceLabels: { annual: '매년', monthly: '매월', none: '자동 부여 없음' }, annualAmount: '연간 부여 일수', monthlyAmount: '월별 부여 일수', expiryMode: '소멸 방식', expiryModeLabels: { fiscalYearEnd: '회계연도 말', monthsAfterGrant: '부여 후 개월', none: '없음' }, expiryMonths: '소멸 개월', carryover: '이월 허용', carryoverDescription: '남은 잔여량을 다음 기간으로 넘길 수 있게 합니다.', carryoverLimit: '이월 한도', carryoverLimitDescription: '비워 두면 한도를 두지 않습니다.', usageLimit: '사용 최대 한도', usageLimitDescription: '한 해에 이만큼까지 쓸 수 있습니다. 비워 두면 한도를 두지 않습니다.', yearStartConfirmationTitle: '휴가 연도 시작일을 바꿀까요?', yearStartConfirmationDescription: '이미 기록된 휴가가 속한 연도가 바뀝니다. 남은 일수도 그 기준으로 다시 계산됩니다.', expiryConfirmationTitle: '소멸·이월 정책을 변경할까요?', expiryConfirmationDescription: '저장하면 현재 남아 있는 휴가에도 새 소멸일과 이월 규칙이 바로 적용됩니다.', expiryConfirmationCancel: '취소', expiryConfirmationSave: '변경 저장', remove: '삭제', removeConfirmationTitle: '휴가 종류를 삭제할까요?', removeConfirmationDescription: '‘{name}’ 휴가 종류는 신규 신청과 설정 목록에서 삭제됩니다. 기존 사용 내역은 유지됩니다.', removeConfirmationCancel: '취소', removeConfirmationAction: '삭제', removeSuccess: '휴가 종류를 삭제했습니다.', removeError: '휴가 종류를 삭제하지 못했습니다.', cancelChanges: '변경 취소', save: '저장', loading: '불러오는 중...', loadError: '휴가 설정을 불러오지 못했습니다.', saveError: '휴가 설정을 저장하지 못했습니다.', saveSuccess: '휴가 설정을 저장했습니다.', atLeastOneUnit: '신청 단위를 하나 이상 선택하세요.', requiredName: '이름을 입력하세요.', invalidPolicy: '정책 값을 다시 확인하세요.'
		},
		users: {
			member: '일반',
			admin: '관리자'
		},
		organization: organizationText.ko
	},
	en: {
		messages: {
			userSaveError: 'Could not save the user.'
		},
		companyHolidays: {
			title: 'Company holidays', description: 'Add company anniversaries and office closure days to every member calendar and leave calculation.', add: 'Add holiday', createTitle: 'Add company holiday', editTitle: 'Edit company holiday', name: 'Holiday name', namePlaceholder: 'e.g. Company anniversary', date: 'Date', recursAnnually: 'Repeat annually', recurrenceDescription: 'Show this holiday on the same month and day each year. Weekend overlaps do not create an automatic substitute holiday.', annual: 'Repeats annually', oneTime: 'One time', edit: 'Edit', remove: 'Delete', cancel: 'Cancel', save: 'Save', loading: 'Loading...', empty: 'No company holidays have been added.', loadError: 'Could not load company holidays.', saveError: 'Could not save the company holiday.', saveSuccess: 'Company holiday saved.', removeError: 'Could not delete the company holiday.', removeSuccess: 'Company holiday deleted.', removeConfirmationTitle: 'Delete this company holiday?', removeConfirmationDescription: 'It will no longer appear on member calendars or apply to leave calculations.'
		},
		workSettings: {
			title: 'Work settings',
			description: 'Manage the company work mode and work-time calculation rules.',
			workMode: 'Work mode',
			workModeDescription: 'Choose the single work mode used by the company.',
			autonomous: 'Autonomous',
			autonomousDescription: 'Record actual work time without a baseline.',
			flexible: 'Flexible',
			flexibleDescription: 'Work freely within weekly targets and core hours.',
			fixed: 'Fixed',
			fixedDescription: 'Apply fixed start and end times on selected weekdays.',
			workingWeekdays: 'Working weekdays',
			weekdays: ['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun'],
			hourUnit: 'h',
			minuteUnit: 'm',
			start: 'start',
			end: 'end',
			weeklyTarget: 'Weekly target',
			referenceStart: 'Leave reference start',
			coreTime: 'Core hours',
			coreTimeEnabled: 'Use core hours',
			fixedHours: 'Start and end time',
			commonRules: 'Common rules',
			commonRulesDescription: 'Applied to actual work and night-work calculations.',
			breakPeriods: 'Break periods',
			addBreak: 'Add break',
			nightHours: 'Night-work hours',
			remove: 'Remove',
			preview: 'Employee baseline preview',
			previewDescription: 'Applied to My work time and employee work status.',
			dailyTarget: 'Daily target',
			weeklyTargetPreview: 'Weekly target',
			monthlyTarget: 'This month',
			noBaseline: 'No baseline',
			autonomousNotice: 'Employees see actual work and night work without a baseline.',
			save: 'Save changes',
			loading: 'Loading work settings...',
			loadError: 'Could not load work settings.',
			saveError: 'Could not save work settings.',
			saveSuccess: 'Work settings saved.',
			invalidWeekdays: 'Select at least one working weekday.',
			invalidTarget: 'Check the target work time.'
		},
		attendanceSettings: {
			title: 'Leave settings', description: 'Manage default and custom leave types for your company.', systemBadge: 'Default leave', add: 'Add leave type', newLeave: 'New leave type', unsaved: 'Unsaved', inactive: 'Stopped', paid: 'Paid', paidDescription: 'Choose whether this leave type is paid.', unpaid: 'Unpaid', balanceMode: 'Balance mode', allowedUnits: 'Request units', allowedUnitsDescription: 'Offering a smaller unit offers the larger ones with it.', fullDay: 'Full day', halfDay: 'Half day', quarterDay: 'Quarter day', dayUnit: 'days', increaseDays: 'Increase days', decreaseDays: 'Decrease days', name: 'Name', namePlaceholder: 'e.g. Family care leave', systemKind: 'System kind', balanceModeAnnual: 'Deduct annual balance', balanceModeSeparate: 'Separate balance', balanceModeNone: 'No balance', balanceModeUnavailable: 'not available here', includeInSummary: 'Include in leave summary', includeInSummaryDescription: 'Include this balance in the employee attendance summary totals.', annualBalanceDescription: 'This leave type deducts the annual leave balance without creating a separate balance.', grantCadence: 'Grant cadence', grantCadenceLabels: { annual: 'Annual', monthly: 'Monthly', none: 'No automatic grant' }, annualAmount: 'Annual grant days', monthlyAmount: 'Monthly grant days', expiryMode: 'Expiry mode', expiryModeLabels: { fiscalYearEnd: 'Fiscal year end', monthsAfterGrant: 'Months after grant', none: 'None' }, expiryMonths: 'Expiry months', carryover: 'Allow carryover', carryoverDescription: 'Carry remaining balance into the next period.', carryoverLimit: 'Carryover limit', carryoverLimitDescription: 'Leave blank for no limit.', usageLimit: 'Usage limit', usageLimitDescription: 'The most that can be taken in a year. Leave blank for no limit.', yearStartConfirmationTitle: 'Change the day the leave year starts?', yearStartConfirmationDescription: 'Leave already recorded moves to the year it now falls in, and remaining days are counted from that day.', expiryConfirmationTitle: 'Change the expiry and carryover policy?', expiryConfirmationDescription: 'Saving immediately applies the new expiry dates and carryover rules to current available leave.', expiryConfirmationCancel: 'Cancel', expiryConfirmationSave: 'Save changes', remove: 'Delete', removeConfirmationTitle: 'Delete this leave type?', removeConfirmationDescription: '{name} will be deleted from new requests and settings. Existing history will be preserved.', removeConfirmationCancel: 'Cancel', removeConfirmationAction: 'Delete', removeSuccess: 'Leave type deleted.', removeError: 'Could not delete leave type.', cancelChanges: 'Cancel changes', save: 'Save', loading: 'Loading...', loadError: 'Could not load leave settings.', saveError: 'Could not save leave settings.', saveSuccess: 'Leave settings saved.', atLeastOneUnit: 'Select at least one request unit.', requiredName: 'Enter a name.', invalidPolicy: 'Review the policy values.'
		},
		users: {
			member: 'Member',
			admin: 'Admin'
		},
		organization: organizationText.en
	}
} as const;
