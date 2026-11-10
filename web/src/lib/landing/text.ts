export const landingText = {
	ko: {
		title: '김인턴 · AI 동료와 쓰는 작은 회사의 백오피스',
		description: '근태, 업무, 일정, 고객, 문서를 한 앱에서 관리합니다. 메신저로 부탁하면 김인턴이 회사 기록을 보고 처리합니다.',
		brand: '김인턴',
		navigation: { docs: '문서', github: 'GitHub', signIn: '로그인', changeLanguage: '언어 바꾸기' },
		hero: {
			headline: 'AI 동료와 쓰는 백오피스',
			lead: '근태, 업무, 일정, 고객, 문서를 한 앱에서 관리합니다. 메신저로 부탁하면 김인턴이 회사 기록을 보고 처리하고, 그 기록은 투자자에게 바로 보낼 수 있는 데이터룸이 됩니다.',
			start: '회사 만들기',
			signIn: '로그인',
			facts: ['Apache-2.0 오픈 소스', '대화는 회사 컴퓨터에', 'AI 사용료는 원가 그대로']
		},
		demo: {
			label: '김인턴과 나눈 대화 예시',
			conversation: '김인턴',
			working: '김인턴이 처리하는 중',
			scenarios: [
				{
					id: 'attendance',
					turns: [
						{ from: 'person', text: '오늘 재택이야, 출근 기록해 줘.', file: '', steps: [], link: { url: '', siteName: '', title: '' } },
						{ from: 'agent', text: '재택 출근 기록을 남겼습니다. 2026-11-09(월) 08:13, 재택 기준으로 저장되었습니다.', file: '', steps: ['equip', 'company_settings_get', 'attendance_add'], link: { url: '', siteName: '', title: '' } }
					]
				},
				{
					id: 'calendar',
					turns: [
						{ from: 'person', text: '다음 주에 박예시님이랑 나 둘 다 되는 한 시간 찾아서 회의 잡아 줘.', file: '', steps: [], link: { url: '', siteName: '', title: '' } },
						{ from: 'agent', text: '두 분 모두 다음 주에 겹치지 않는 시간을 찾아 회의를 등록했습니다.\n\n- **일시**: 11월 19일(목) 14:00~15:00 (1시간)\n- **참석자**: 이샘플, 박예시\n- **일정명**: 팀 미팅 (이샘플·박예시)', file: '', steps: ['plan', 'person_list', 'event_list', 'event_add(팀 미팅)'], link: { url: '', siteName: '', title: '' } }
					]
				},
				{
					id: 'files',
					turns: [
						{ from: 'person', text: '이거 회사 양식 견적서 PDF로 만들어 줘.', file: '견적_항목.xlsx', steps: [], link: { url: '', siteName: '', title: '' } },
						{ from: 'agent', text: '이샘플님, 결과부터 말씀드립니다. 견적서 PDF(견적서_예시상사.pdf)를 첨부합니다.\n\n현재 파일 상태는 다음과 같습니다.\n\n- 문서번호: Q-2026-0142 / 견적일자 2026-11-08 / 유효기간 30일\n- 공급가액 4,690,000원, 부가세 469,000원, 총 5,159,000원', file: '견적서_예시상사.pdf', steps: ['plan', 'bash(office read 견적_항목.xlsx)', 'company_info_get', 'bash(office merge kr/quote)'], link: { url: '', siteName: '', title: '' } }
					]
				},
				{
					id: 'dataRoom',
					turns: [
						{ from: 'person', text: '투자사에서 회사 자료를 요청했어. 투자자 범위로 보기 전용 데이터룸 링크 만들어 줘.', file: '', steps: [], link: { url: '', siteName: '', title: '' } },
						{ from: 'agent', text: '투자자 범위로 다운로드 없이 보기만 가능하고 72시간 후 만료되는 데이터룸 링크를 만들까요?', file: '', steps: ['dataroom_get', 'circle_list'], link: { url: '', siteName: '', title: '' } },
						{ from: 'person', text: '응, 만들어 줘.', file: '', steps: [], link: { url: '', siteName: '', title: '' } },
						{ from: 'agent', text: '보기 전용 데이터룸 링크를 만들었습니다.\n\n- **링크**: https://intern.kim/share/links/4f1c7a2e-9b3d-4e8a-a6f0-2d5c8e17b934\n- **접속 코드**: 622332\n- **범위**: 투자자 서클\n- **권한**: 보기 전용 (다운로드 불가)\n- **유효 기간**: 72시간', file: '', steps: ['dataroom_link_add'], link: { url: 'https://intern.kim/share/links/4f1c7a2e-9b3d-4e8a-a6f0-2d5c8e17b934', siteName: 'intern.kim', title: '데이터룸' } }
					]
				}
			]
		},
		day: {
			title: '김인턴과 보내는 하루',
			lead: '메신저로 부탁하면 김인턴이 회사 기록을 보고 처리합니다. 결과는 팀이 쓰는 화면에 그대로 남습니다.',
			moments: [
				{
					section: 'attendance',
					time: '08:13',
					note: '',
					caption: '답은 팀이 보는 근태 기록에서 나옵니다.',
					turns: [
						{ from: 'person', text: '오늘 누구누구 출근했어?' },
						{ from: 'agent', text: '오늘 6명 중 4명이 근무 중입니다.\n\n- **근무 중**: 이샘플, 박예시, 정예시 (제품팀), 최견본 (디자인팀)\n- **휴가**: 김예시\n- **출근 전**: 한샘플' }
					]
				},
				{
					section: 'calendar',
					time: '09:30',
					note: '',
					caption: '일정을 물으면 팀 캘린더를 보고 답합니다.',
					turns: [
						{ from: 'person', text: '이번 주 내 일정 알려 줘.' },
						{ from: 'agent', text: '이번 주 일정은 4개입니다.\n\n- **화 10:00** 주간 회의\n- **수 14:00** 예시상사 미팅\n- **목 12:00** 팀 점심\n- **금 16:00** 투자사 미팅' }
					]
				},
				{
					section: 'task',
					time: '11:20',
					note: '',
					caption: '맡긴 일은 팀의 업무 보드에 올라갑니다.',
					turns: [
						{ from: 'person', text: '예시상사 견적서 회신을 박예시님 업무로 올려 줘.' },
						{ from: 'agent', text: '박예시님 업무로 등록했습니다.\n\n- **업무**: 예시상사 견적서 회신\n- **사업**: 영업\n- **상태**: 예정' }
					]
				},
				{
					section: 'messenger',
					time: '14:10',
					caption: '팀 채널에서 동료와 이야기하다가 김인턴을 부르면, 그 자리에서 일을 받습니다.',
					note: '채널, 1:1 대화, 스레드, 파일 공유를 갖춘 사내 메신저입니다. 김인턴이 처리한 결과는 채널에 있는 모두가 함께 봅니다.',
					turns: []
				},
				{
					section: 'dataRoom',
					time: '17:40',
					note: '',
					caption: '기록한 지표는 투자자가 보는 데이터룸에 바로 반영됩니다.',
					turns: [
						{ from: 'person', text: '지난달 매출 4,800만 원으로 기록해 줘.' },
						{ from: 'agent', text: '지난달 월매출을 4,800만 원으로 기록했습니다. 전월보다 6.7% 늘었습니다.' }
					]
				}
			]
		},
		channel: {
			name: '영업',
			viewer: '이샘플',
			messages: [
				{ sender: '박예시', text: '예시상사 미팅 끝났어요. 다음 주에 시안을 먼저 보고 싶다고 하네요.' },
				{ sender: '이샘플', text: '@김인턴 방금 미팅 내용 CRM에 남기고, 다음 주 시안 공유를 내 업무로 올려 줘.' },
				{ sender: '김인턴', text: 'CRM에 미팅 기록을 남기고 업무를 등록했습니다.\n\n- **고객**: 예시상사\n- **활동**: 미팅, 다음 주 시안 검토 요청\n- **업무**: 예시상사 시안 공유 (이샘플)' },
				{ sender: '박예시', text: '고마워요 👍' }
			]
		},
		investors: {
			title: '투자자가 원하는 자료를 준비 없이 보냅니다',
			lead: '일하며 남긴 기록과 김인턴이 만든 문서가 데이터룸에 분류되어 쌓입니다. 자료 요청이 오면 받는 사람에게 맞는 범위로 링크를 보냅니다.',
			company: {
				title: '회사는',
				items: [
					{ title: '자료를 따로 모으지 않습니다', body: '김인턴이 만든 계약서, 견적서, 보고서는 분류되어 데이터룸에 보관됩니다.' },
					{ title: '회사 소개가 기록에서 만들어집니다', body: '매출 같은 지표와 연혁, 지분 구조를 기록하면 데이터룸의 회사 소개에 그래프와 표로 나옵니다.' },
					{ title: '받는 사람마다 범위를 고릅니다', body: '링크는 서클 하나가 읽는 분류만 엽니다. 투자자, 회계·세무 자문, 법률 자문, 금융기관 서클이 처음부터 있습니다.' }
				]
			},
			investor: {
				title: '투자자는',
				items: [
					{ title: '링크와 코드로 바로 엽니다', body: '계정 없이 6자리 코드를 넣고 기밀유지 안내에 동의하면 됩니다.' },
					{ title: '열 때마다 지금의 기록을 봅니다', body: '링크는 열 때마다 데이터룸을 새로 읽습니다. 그사이 기록한 지표와 문서가 바로 보입니다.' },
					{ title: '필요한 만큼 깊이 봅니다', body: '실사가 진행되어 회사가 서클 범위를 넓히면, 같은 링크에서 더 많은 분류를 이어서 봅니다.' }
				]
			},
			scope: '서클마다 열리는 분류',
			guard: '링크는 길어야 7일 열리고 원본 다운로드는 따로 허용합니다. 내려받은 PDF의 모든 쪽에는 어느 링크로 언제 받았는지 찍힙니다.'
		},
		ownership: {
			title: '회사의 데이터는 회사가 쥡니다',
			lead: '대화와 작업 파일은 처음부터 회사 컴퓨터에 있습니다. 기록과 앱까지 직접 운영할지는 회사의 단계에 맞춰 정합니다.',
			table: {
				caption: '무엇이 어디에 있는지',
				what: '무엇',
				hosted: 'intern.kim과 함께',
				selfHosted: '전부 직접',
				rows: [
					{ what: 'AI 동료, 대화, 작업 파일', hosted: '회사 컴퓨터', selfHosted: '회사 컴퓨터' },
					{ what: '회사 기록과 파일 저장소', hosted: 'intern.kim', selfHosted: '회사의 Supabase' },
					{ what: '웹 앱과 로그인 주소', hosted: 'intern.kim', selfHosted: '회사의 도메인' }
				],
				note: 'intern.kim의 회사 기록에는 메시지를 저장하는 테이블이 없습니다. 대화는 회사 컴퓨터에만 저장됩니다.'
			},
			items: [
				{ title: '운영이 멈춰도 계속 씁니다', body: '모든 소프트웨어가 Apache-2.0 오픈 소스입니다. 여명거리가 서비스를 멈춰도 회사가 직접 운영하며 계속 쓸 수 있습니다.' },
				{ title: '보안을 직접 검증하고 고칩니다', body: '코드를 누구나 읽을 수 있습니다. 회사나 보안 감사인이 직접 검증하고, 찾은 문제는 고쳐서 배포할 수 있습니다.' },
				{ title: '부탁한 사람의 권한으로만', body: '김인턴은 부탁한 사람이 볼 수 있는 기록만 읽고, 할 수 있는 일만 합니다. 남긴 기록에는 그 사람의 이름이 붙습니다.' },
				{ title: '되돌리기 어려운 일은 먼저 묻습니다', body: '데이터룸 링크 만들기처럼 되돌리기 어려운 일은 부탁한 사람에게 먼저 확인합니다.' }
			],
			source: 'GitHub에서 코드 보기'
		},
		plans: {
			title: '사내 AI 동료를 과한 비용 없이',
			lead: '서버를 빌리거나 개발자를 두지 않아도 됩니다. 회사에 둔 컴퓨터 한 대에서 김인턴이 모든 구성원의 부탁을 처리합니다.',
			pay: {
				title: '구독과 AI 사용료',
				items: [
					{ title: '좌석 구독', body: '구성원 수만큼 구독합니다. 구독에 AI 크레딧이 들어 있고, 모자라면 더 살 수 있습니다.' },
					{ title: '원가 그대로', body: 'AI 호출은 실제 비용만큼 크레딧에서 빠집니다. 김인턴은 일마다 맞는 모델을 골라 씁니다.' },
					{ title: '회사 OpenRouter 키', body: 'OpenRouter 키를 연결하면 모델 비용을 직접 냅니다.' }
				]
			},
			run: {
				title: 'AI 동료가 일할 컴퓨터',
				items: [
					{ title: '김미니', body: '김인턴이 설치된 채로 도착하는 작은 컴퓨터입니다. 전원과 인터넷만 연결하면 됩니다.' },
					{ title: '회사 컴퓨터', body: '늘 켜 둘 리눅스 서버나 Mac에 명령 두 줄로 설치합니다.' },
					{ title: '직접 운영', body: '앱과 데이터베이스까지 회사가 직접 운영합니다.' }
				],
				guide: '셀프호스팅 안내',
				kimMini: '김미니 세 대. 윗면에 김인턴 얼굴이 새겨진 은색 정사각형 상자에 짧은 안테나가 달려 있습니다.'
			}
		},
		closing: { title: '회사를 만들고 동료를 초대하세요', quickstart: '빠른 시작 읽기' },
		footer: { owner: '여명거리', rights: '모든 권리 보유.', license: 'Apache-2.0' }
	},
	en: {
		title: 'Intern Kim · A back office with an AI coworker',
		description: 'Attendance, tasks, calendar, customers and documents in one app. Ask in the messenger, and Intern Kim reads your company records and does the work.',
		brand: 'Intern Kim',
		navigation: { docs: 'Docs', github: 'GitHub', signIn: 'Sign in', changeLanguage: 'Change language' },
		hero: {
			headline: 'A back office with an AI coworker',
			lead: 'Attendance, tasks, calendar, customers and documents in one app. Ask in the messenger and Intern Kim does the work from your company records, and those records become a data room you can send to investors.',
			start: 'Create a company',
			signIn: 'Sign in',
			facts: ['Open source under Apache-2.0', 'Conversations stay on your computer', 'AI billed at cost']
		},
		demo: {
			label: 'A sample conversation with Intern Kim',
			conversation: 'Intern Kim',
			working: 'Intern Kim is working on it',
			scenarios: [
				{
					id: 'attendance',
					turns: [
						{ from: 'person', text: 'Clock me in — I\'m working from home today.', file: '', steps: [], link: { url: '', siteName: '', title: '' } },
						{ from: 'agent', text: 'You\'re clocked in as of 08:13 KST at Remote.', file: '', steps: ['equip', 'company_settings_get', 'attendance_add'], link: { url: '', siteName: '', title: '' } }
					]
				},
				{
					id: 'calendar',
					turns: [
						{ from: 'person', text: 'Find an hour next week when Park Example and I are both free, and book it.', file: '', steps: [], link: { url: '', siteName: '', title: '' } },
						{ from: 'agent', text: 'Done. I found an hour next week when you\'re both free and booked it:\n\n- **When**: Thu, Nov 19, 14:00–15:00 (1 hour)\n- **Attendees**: Lee Sample, Park Example\n- **Title**: Team meeting (Lee · Park)', file: '', steps: ['plan', 'person_list', 'event_list', 'event_add(Team meeting)'], link: { url: '', siteName: '', title: '' } }
					]
				},
				{
					id: 'files',
					turns: [
						{ from: 'person', text: 'Turn this into a quote PDF on our company letterhead.', file: 'quote-items.xlsx', steps: [], link: { url: '', siteName: '', title: '' } },
						{ from: 'agent', text: 'The quote is attached and filed in the data room. Details:\n\n- Number: Q-2026-0142 · dated 2026-11-08 · valid for 30 days\n- Supply 4,690,000 KRW, VAT 469,000 KRW, total 5,159,000 KRW', file: 'quote-example-co.pdf', steps: ['plan', 'bash(office read quote-items.xlsx)', 'company_info_get', 'bash(office merge kr/quote)'], link: { url: '', siteName: '', title: '' } }
					]
				},
				{
					id: 'dataRoom',
					turns: [
						{ from: 'person', text: 'An investor asked for our company materials. Make a view-only data room link for the investor circle.', file: '', steps: [], link: { url: '', siteName: '', title: '' } },
						{ from: 'agent', text: 'Create a view-only data room link for the investor circle that expires after 72 hours and does not allow downloads?', file: '', steps: ['dataroom_get', 'circle_list'], link: { url: '', siteName: '', title: '' } },
						{ from: 'person', text: 'Yes, create it.', file: '', steps: [], link: { url: '', siteName: '', title: '' } },
						{ from: 'agent', text: 'The link is ready.\n\n- **Link**: https://intern.kim/share/links/4f1c7a2e-9b3d-4e8a-a6f0-2d5c8e17b934\n- **Access code**: 622332\n- **Access**: read-only through the Investor circle. Downloading originals is disabled.\n- **Valid for 72 hours**: recipients enter the code and accept the confidentiality notice without an account.\n\nShare the link and code with them by your preferred channel.', file: '', steps: ['dataroom_link_add'], link: { url: 'https://intern.kim/share/links/4f1c7a2e-9b3d-4e8a-a6f0-2d5c8e17b934', siteName: 'intern.kim', title: 'Dataroom' } }
					]
				}
			]
		},
		day: {
			title: 'A day with Intern Kim',
			lead: 'Ask in the messenger and Intern Kim works from your company records. The result stays on the screens your team already uses.',
			moments: [
				{
					section: 'attendance',
					time: '08:13',
					note: '',
					caption: 'The answer comes from the attendance records your team sees.',
					turns: [
						{ from: 'person', text: "Who's in today?" },
						{ from: 'agent', text: '4 of 6 people are working today.\n\n- **Working**: Lee Sample, Park Example, Jung Example (Product), Choi Specimen (Design)\n- **On leave**: Kim Example\n- **Not in yet**: Han Sample' }
					]
				},
				{
					section: 'calendar',
					time: '09:30',
					note: '',
					caption: "Ask about your schedule and it answers from the team calendar.",
					turns: [
						{ from: 'person', text: "What's on my calendar this week?" },
						{ from: 'agent', text: 'You have 4 events this week.\n\n- **Tue 10:00** Weekly meeting\n- **Wed 14:00** Example Co. meeting\n- **Thu 12:00** Team lunch\n- **Fri 16:00** Investor meeting' }
					]
				},
				{
					section: 'task',
					time: '11:20',
					note: '',
					caption: "Work you hand over lands on the team's task board.",
					turns: [
						{ from: 'person', text: "Put the reply to Example Co. on the quote on Park Example's list." },
						{ from: 'agent', text: "Added it to Park Example's tasks.\n\n- **Task**: Reply to Example Co. on the quote\n- **Business**: Sales\n- **Status**: Planned" }
					]
				},
				{
					section: 'messenger',
					time: '14:10',
					caption: 'Mention Intern Kim in a team channel and it picks up the work right there.',
					note: 'A company messenger with channels, direct messages, threads and file sharing. Everyone in the channel sees what Intern Kim did.',
					turns: []
				},
				{
					section: 'dataRoom',
					time: '17:40',
					note: '',
					caption: 'Metrics you record show up in the data room investors see.',
					turns: [
						{ from: 'person', text: "Record last month's revenue as $35,500." },
						{ from: 'agent', text: "Recorded last month's revenue: $35,500, up 6.0% from the month before." }
					]
				}
			]
		},
		channel: {
			name: 'sales',
			viewer: 'Lee Sample',
			messages: [
				{ sender: 'Park Example', text: 'Just finished the Example Co. meeting. They want to see a draft next week.' },
				{ sender: 'Lee Sample', text: '@Intern Kim log that meeting in the CRM and put sharing the draft next week on my list.' },
				{ sender: 'Intern Kim', text: 'Logged the meeting in the CRM and added the task.\n\n- **Customer**: Example Co.\n- **Activity**: Meeting, asked to review a draft next week\n- **Task**: Share the draft with Example Co. (Lee Sample)' },
				{ sender: 'Park Example', text: 'Thanks 👍' }
			]
		},
		investors: {
			title: 'Send investors what they ask for, with nothing to prepare',
			lead: 'The records your work leaves and the documents Intern Kim writes are filed in the data room as you go. When someone asks for materials, send a link scoped to who they are.',
			company: {
				title: 'For your company',
				items: [
					{ title: 'Nothing to gather', body: 'Contracts, quotes and reports Intern Kim writes are filed in the data room.' },
					{ title: 'The company profile builds itself', body: 'Record metrics like revenue, your history and your cap table, and the data room shows them as charts and tables.' },
					{ title: 'A scope for each recipient', body: 'A link opens only the categories one circle reads. Investor, accountant, legal adviser and lender circles are there from day one.' }
				]
			},
			investor: {
				title: 'For investors',
				items: [
					{ title: 'Open it with a link and a code', body: 'No account. Enter the six-digit code and accept the confidentiality notice.' },
					{ title: 'Always the current records', body: 'The link reads the data room each time it opens, so new metrics and documents show up right away.' },
					{ title: 'As deep as you need', body: 'When diligence moves on and the company widens the circle, the same link shows more categories.' }
				]
			},
			scope: 'What each circle opens',
			guard: 'Links last at most 7 days, and downloading originals is a separate choice. Every page of a downloaded PDF is stamped with the link and the time.'
		},
		ownership: {
			title: 'Your company keeps its data',
			lead: 'Conversations and working files live on your company computer from the start. How much more you run yourself is up to you.',
			table: {
				caption: 'What lives where',
				what: 'What',
				hosted: 'With intern.kim',
				selfHosted: 'All yours',
				rows: [
					{ what: 'AI coworker, conversations, working files', hosted: 'Your computer', selfHosted: 'Your computer' },
					{ what: 'Company records and file storage', hosted: 'intern.kim', selfHosted: 'Your Supabase' },
					{ what: 'Web app and sign-in address', hosted: 'intern.kim', selfHosted: 'Your domain' }
				],
				note: 'The company record on intern.kim has no table for messages. Conversations are stored only on your company computer.'
			},
			items: [
				{ title: 'Keeps running if we stop', body: 'All software is open source under Apache-2.0. If Yeomyeonggeori stops the service, your company can run it and keep going.' },
				{ title: 'Audit and patch it yourself', body: 'Anyone can read the code. Your team or a security auditor can verify it, fix what they find and deploy the fix.' },
				{ title: 'Only with your permissions', body: 'Intern Kim reads only the records the person asking can see, and does only what they can do. What it records carries their name.' },
				{ title: 'Asks before anything hard to undo', body: 'Before something hard to undo, like creating a data room link, it asks the person who made the request.' }
			],
			source: 'Read the code on GitHub'
		},
		plans: {
			title: 'An in-house AI coworker without the big bill',
			lead: 'No servers to rent and no developer to hire. One computer at your company handles requests from everyone on the team.',
			pay: {
				title: 'Seats and AI usage',
				items: [
					{ title: 'Per seat', body: 'Pay per member. AI credits are included, and you can buy more when you run low.' },
					{ title: 'At cost', body: 'Each AI call takes its actual cost from your credits. Intern Kim picks the right model for each job.' },
					{ title: 'Your OpenRouter key', body: 'Connect it and pay model costs directly.' }
				]
			},
			run: {
				title: 'A computer for your AI coworker',
				items: [
					{ title: 'Kim mini', body: 'A small computer that arrives with Intern Kim installed. Connect power and internet.' },
					{ title: 'Your computer', body: 'Install with two commands on a Linux server or Mac that stays on.' },
					{ title: 'Self-hosting', body: 'Run the app and database yourself.' }
				],
				guide: 'Self-hosting guide',
				kimMini: 'Three Kim minis: square silver boxes with the Intern Kim face on top and a short antenna.'
			}
		},
		closing: { title: 'Create a company and invite your team', quickstart: 'Read the quickstart' },
		footer: { owner: 'Yeomyeonggeori', rights: 'All rights reserved.', license: 'Apache-2.0' }
	}
} as const;
