export type CalendarGoogleOAuthClientGuideText = {
	title: string;
	intro: string;
	checklistTitle: string;
	checks: string[];
	stepsTitle: string;
	steps: Array<{
		title: string;
		body: string;
		action?: {
			label: string;
			url: string;
		};
	}>;
	redirectURI: string;
	javascriptOrigin: string;
};

export const googleOAuthClientGuideText: Record<'ko' | 'en', CalendarGoogleOAuthClientGuideText> = {
	ko: {
		title: 'client.json 만드는 방법',
		intro:
			'이미 client.json 파일이 있으면 바로 업로드하세요. 처음 설정하는 경우 아래 순서대로 Google Cloud에서 OAuth 클라이언트를 만들고 JSON 파일을 다운로드합니다.',
		checklistTitle: '진행 전 확인하세요.',
		checks: [
			'Google Cloud 오른쪽 위 프로필이 회사 관리자 계정인지 확인하세요.',
			'개인 Gmail이나 다른 조직 계정이면 계정을 전환한 뒤 진행하세요.',
			'상단 프로젝트가 internkim-calendar인지 확인하세요.',
			'프로젝트 소유자가 개인 계정이 아니라 회사 조직인지 확인하세요.'
		],
		stepsTitle: '설정 순서',
		steps: [
			{
				title: 'Google Cloud 프로젝트를 선택하거나 새로 만듭니다.',
				body: '조직 소유 프로젝트를 사용하세요. 새로 만들 경우 프로젝트 이름은 internkim-calendar처럼 알아보기 쉽게 지정합니다.',
				action: {
					label: '프로젝트 만들기',
					url: 'https://console.cloud.google.com/projectcreate'
				}
			},
			{
				title: 'CalDAV API를 사용 설정합니다.',
				body: '프로젝트가 맞는지 다시 확인한 뒤 CalDAV API 사용 버튼을 누릅니다.',
				action: {
					label: 'CalDAV API 활성화 열기',
					url: 'https://console.cloud.google.com/marketplace/product/google/caldav.googleapis.com'
				}
			},
			{
				title: 'OAuth 동의 화면을 확인합니다.',
				body: '기업 계정만 연결할 예정이면 사용자 유형은 내부용으로 설정합니다. 외부용으로 만들면 불필요한 검토나 경고가 생길 수 있습니다.'
			},
			{
				title: 'OAuth 클라이언트를 만들거나 기존 클라이언트를 엽니다.',
				body: '애플리케이션 유형은 웹 애플리케이션을 선택하세요. 기존 클라이언트가 있으면 새로 만들지 말고 기존 항목을 열어 수정합니다.',
				action: {
					label: 'OAuth 클라이언트 열기',
					url: 'https://console.cloud.google.com/auth/clients'
				}
			},
			{
				title: '승인된 리디렉션 URI를 등록합니다.',
				body: '아래 주소를 OAuth 클라이언트의 승인된 리디렉션 URI에 추가합니다.'
			},
			{
				title: 'JSON 파일을 다운로드합니다.',
				body: 'OAuth 클라이언트 상세 화면에서 JSON 다운로드를 누릅니다. 비밀번호가 표시되면 안전한 곳에 따로 저장하세요. 이 화면을 벗어나면 다시 보이지 않을 수 있습니다. 다운로드한 파일 이름이 달라도 이 화면에는 해당 JSON 파일을 그대로 업로드하면 됩니다.'
			},
			{
				title: '업로드 후 연결 버튼을 확인합니다.',
				body: '업로드가 끝나면 이 화면이 Google 캘린더를 연결하세요 상태로 바뀌고 연결 버튼이 나타납니다.'
			}
		],
		redirectURI: '승인된 리디렉션 URI',
		javascriptOrigin: '승인된 JavaScript 원본'
	},
	en: {
		title: 'How to create client.json',
		intro:
			'If you already have client.json, upload it here. For first-time setup, create an OAuth client in Google Cloud and download the JSON file.',
		checklistTitle: 'Check before continuing.',
		checks: [
			'Confirm the Google Cloud profile in the top right is the company admin account.',
			'Switch accounts first if you are using a personal Gmail account or another organization account.',
			'Confirm the selected project is internkim-calendar.',
			'Confirm the project is owned by the company organization, not a personal account.'
		],
		stepsTitle: 'Setup steps',
		steps: [
			{
				title: 'Select or create the Google Cloud project.',
				body: 'Use an organization-owned project. If you create a new one, use a recognizable name such as internkim-calendar.',
				action: {
					label: 'Open project creation',
					url: 'https://console.cloud.google.com/projectcreate'
				}
			},
			{
				title: 'Enable the CalDAV API.',
				body: 'Confirm the project again, then enable the CalDAV API.',
				action: {
					label: 'Open CalDAV API',
					url: 'https://console.cloud.google.com/marketplace/product/google/caldav.googleapis.com'
				}
			},
			{
				title: 'Review the OAuth consent screen.',
				body: 'If only company accounts should connect, set the user type to internal. External apps can introduce unnecessary review steps or warnings.'
			},
			{
				title: 'Create an OAuth client or open the existing one.',
				body: 'Set the application type to web application. If a client already exists, edit that client instead of creating another one.',
				action: {
					label: 'Open OAuth clients',
					url: 'https://console.cloud.google.com/auth/clients'
				}
			},
			{
				title: 'Add the authorized redirect URI.',
				body: 'Add the URI below to the OAuth client authorized redirect URIs.'
			},
			{
				title: 'Download the JSON file.',
				body: 'In the OAuth client detail page, download the JSON file. Save the password somewhere secure if it is displayed. It may not be visible again after you leave the page. The downloaded filename can be different; upload that JSON file here.'
			},
			{
				title: 'Check the connect button after upload.',
				body: 'After upload, this panel changes to Google Calendar guidance and shows the Connect button.'
			}
		],
		redirectURI: 'Authorized redirect URI',
		javascriptOrigin: 'Authorized JavaScript origin'
	}
};
