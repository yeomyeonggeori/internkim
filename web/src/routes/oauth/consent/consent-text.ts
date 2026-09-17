export const consentText = {
	ko: {
		title: '앱 연결',
		reading: '연결 요청을 확인하고 있습니다.',
		asking: '{client}이(가) {email} 계정으로 김인턴에 연결하려고 합니다.',
		grants: '허용하면 이 앱이 회사에서 내가 쓸 수 있는 도구를 내 이름으로 씁니다. 업무, 일정, 메시지, 문서를 읽고 바꿀 수 있습니다.',
		returnsTo: '결정하면 {host}(으)로 돌아갑니다.',
		approve: '허용',
		deny: '거절',
		redirecting: '앱으로 돌아가고 있습니다.',
		missing: '연결 요청이 없습니다. 앱에서 다시 연결을 시작하세요.',
		unreadable: '연결 요청을 읽지 못했습니다. 만료되었을 수 있으니 앱에서 다시 시작하세요.'
	},
	en: {
		title: 'Connect an app',
		reading: 'Checking the connection request.',
		asking: '{client} wants to connect to InternKim as {email}.',
		grants: 'If you allow it, this app uses the tools you can use in your company, in your name. It can read and change tasks, events, messages and documents.',
		returnsTo: 'Either way, you go back to {host}.',
		approve: 'Allow',
		deny: 'Deny',
		redirecting: 'Going back to the app.',
		missing: 'There is no connection request here. Start connecting again from the app.',
		unreadable: 'The connection request could not be read. It may have expired, so start again from the app.'
	}
} as const;
