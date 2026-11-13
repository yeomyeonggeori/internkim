export const landingText = {
	ko: {
		title: '김인턴 · AI 동료와 쓰는 작은 회사의 백오피스',
		description: '근태, 업무, 일정, 고객, 문서를 한 앱에서 관리합니다. 메신저로 부탁하면 김인턴이 회사 기록을 보고 처리합니다.',
		brand: '김인턴',
		navigation: { docs: '문서', github: 'GitHub', signIn: '로그인', changeLanguage: '언어 바꾸기' },
		hero: {
			headline: 'AI 동료와 쓰는 백오피스',
			lead: '근태부터 문서까지 한 앱에서 관리합니다. 메신저로 부탁하면 김인턴이 처리하고, 쌓인 기록은 그대로 투자자에게 보낼 데이터룸이 됩니다.',
			start: '회사 만들기',
			signIn: '로그인',
			facts: ['Apache-2.0 오픈 소스', '대화는 회사 컴퓨터에', 'AI 사용료는 사용한 만큼']
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
			title: '익숙한 화면으로도, 말 한마디로도',
			lead: '늘 쓰던 화면에서 눌러도 되고, 클릭 없이 메신저로 말만 해도 됩니다. 어느 쪽이든 같은 기록에 남습니다.',
			columns: { ask: '김인턴에게 말로', screen: '앱 화면에서 직접' },
			more: {
				title: '이런 일도 맡깁니다',
				items: [
					{ title: '메일', body: '연결한 메일함을 읽고 답할 메일을 추립니다. 보내기 전에는 확인을 받습니다.' },
					{ title: '반복 업무', body: '매주 월요일 9시에 지난주 업무 정리처럼 정해 두면 그때마다 처리합니다.' },
					{ title: '문서', body: '보고서, 엑셀, 발표자료와 회사 양식 문서를 만들고 고칩니다.' },
					{ title: '기억', body: '지난 부탁과 결과를 기억하고 이어서 일합니다. 개인, 서클, 회사의 기억은 따로 보관합니다.' }
				]
			},
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
					caption: '팀 채널에서 김인턴을 부르면 그 자리에서 일을 받습니다.',
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
			lead: '일하며 남긴 기록과 김인턴이 만든 문서가 데이터룸에 쌓입니다. 자료 요청이 오면 받는 사람에 맞는 범위로 링크만 보내세요.',
			company: {
				title: '회사는',
				items: [
					{ title: '자료를 따로 모으지 않습니다', body: '김인턴이 회사 양식으로 만든 견적서, 계약서, 청구서는 문서번호를 받아 데이터룸에 올라갑니다.' },
					{ title: '회사 소개가 기록에서 만들어집니다', body: '매출 같은 지표와 연혁, 지분 구조를 기록하면 데이터룸의 회사 소개에 그래프와 표로 나옵니다.' },
					{ title: '받는 사람마다 범위를 고릅니다', body: '링크는 서클 하나가 읽는 분류만 엽니다. 투자자, 회계·세무 자문, 법률 자문, 금융기관 서클이 처음부터 있습니다.' }
				]
			},
			investor: {
				title: '투자자는',
				items: [
					{ title: '링크와 코드로 바로 엽니다', body: '계정 없이 6자리 코드를 넣고 기밀유지 안내에 동의하면 됩니다.' },
					{ title: '열 때마다 지금의 기록을 봅니다', body: '링크는 열 때마다 데이터룸을 새로 읽습니다. 그사이 기록한 지표와 문서가 바로 보입니다.' },
					{ title: '필요한 만큼 깊이 봅니다', body: '실사가 진행되어 회사가 서클 범위를 넓히면 같은 링크로 더 많은 분류를 봅니다. 링크를 만든 사람이 볼 수 있는 범위까지입니다.' }
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
				selfHosted: '직접 운영',
				rows: [
					{ what: 'AI 동료, 대화, 작업 파일', hosted: '회사 컴퓨터', selfHosted: '회사 컴퓨터' },
					{ what: '회사 기록과 파일 저장소', hosted: 'intern.kim', selfHosted: '회사의 Supabase' },
					{ what: '웹 앱과 로그인 주소', hosted: 'intern.kim', selfHosted: '회사의 도메인' }
				],
				note: 'intern.kim의 회사 기록에는 메시지가 저장되지 않습니다. 김미니의 복구본은 회사 키로 암호화한 뒤에 보관합니다.'
			},
			items: [
				{ title: '운영이 멈춰도 계속 씁니다', body: '김인턴의 소스 코드는 Apache-2.0으로 공개되어 있습니다. 여명거리가 서비스를 멈춰도 회사가 직접 운영하며 계속 쓸 수 있습니다.' },
				{ title: '보안을 직접 검증하고 고칩니다', body: '코드를 누구나 읽을 수 있습니다. 회사나 보안 감사인이 직접 검증하고, 찾은 문제는 고쳐서 배포할 수 있습니다.' },
				{ title: '부탁한 사람의 계정으로 일합니다', body: '구성원마다 회사 컴퓨터에 따로 계정이 있고, 김인턴은 부탁한 사람의 계정과 권한으로 일합니다. 그 사람이 열 수 없는 기록과 파일은 김인턴도 열지 못합니다.' },
				{ title: '보내고 지우고 바꾸기 전에 묻습니다', body: '메일 발송, 휴가 승인, 기록 삭제, 회사 규정 변경은 실행 직전에 멈추고 부탁한 사람의 확인을 기다립니다. 반복 업무는 만들 때 한 번 확인합니다.' }
			],
			source: 'GitHub에서 코드 보기'
		},
		plans: {
			title: '사내 AI 동료를 과한 비용 없이',
			lead: '회사에 둔 컴퓨터 한 대에서 김인턴이 모든 구성원의 부탁을 처리합니다. 따로 빌릴 서버가 없습니다.',
			pay: {
				title: '구독과 AI 사용료',
				items: [
					{ title: '좌석 구독', body: '구성원 수만큼 구독합니다. 구독에 AI 크레딧이 들어 있고, 모자라면 더 살 수 있습니다.' },
					{ title: '사용한 만큼', body: 'AI 호출은 사용한 만큼 크레딧에서 뺍니다. 가벼운 판단은 판단 전용 모델이 맡습니다.' },
					{ title: '회사 OpenRouter 키', body: 'OpenRouter 키를 연결하면 모델 비용을 직접 냅니다.' }
				]
			},
			run: {
				title: 'AI 동료가 일할 컴퓨터',
				items: [
					{ title: '김미니', body: '김인턴이 설치된 채로 도착하는 작은 컴퓨터입니다. 전원과 인터넷을 연결하고 회사에 등록하면 됩니다.' },
					{ title: '김미니 여러 대', body: '회사 키로 암호화한 복구본이 15분마다 저장되어, 한 대가 멈추면 다른 김미니가 이어받을 수 있습니다.' },
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
		title: 'Intern Kim · A back office and AI coworker for small companies',
		description: 'Attendance, tasks, calendar, customers and documents in one app. Ask in the messenger, and Intern Kim reads your company records and does the work.',
		brand: 'Intern Kim',
		navigation: { docs: 'Docs', github: 'GitHub', signIn: 'Sign in', changeLanguage: 'Change language' },
		hero: {
			headline: 'A back office and AI coworker for small companies',
			lead: 'Manage attendance, tasks, calendar, customers and documents in one app. Ask Intern Kim in the company messenger to do the work, and the records add up to a data room you can send investors.',
			start: 'Create a company',
			signIn: 'Sign in',
			facts: ['Open source under Apache-2.0', 'Conversations stay on your company computer', 'AI billed by usage']
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
			title: 'Use the app or ask Intern Kim',
			lead: 'Use the screens you know, or skip the clicks and ask Intern Kim in the messenger. Both work on the same company records.',
			columns: { ask: 'Ask Intern Kim', screen: 'Or do it on screen' },
			more: {
				title: 'More you can hand over',
				items: [
					{ title: 'Mail', body: 'Reads your connected inbox and picks out what needs a reply. It asks before sending.' },
					{ title: 'Recurring work', body: 'Set something like a weekly summary every Monday at 9, and it runs on time.' },
					{ title: 'Documents', body: 'Writes and edits reports, spreadsheets, decks and company forms.' },
					{ title: 'Memory', body: 'Remembers past requests and results and picks up from there. Personal, circle and company memories are kept apart.' }
				]
			},
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
			title: 'Send investors what they ask for, already in order',
			lead: 'Company records and the documents Intern Kim makes collect in your data room as you work. When someone asks, send a link with access set for who they are.',
			company: {
				title: 'For your company',
				items: [
					{ title: 'Documents filed as you work', body: 'Quotes, contracts and invoices Intern Kim makes on your letterhead get a document number and go into the data room.' },
					{ title: 'Company records become charts and tables', body: 'Record metrics like revenue, your history and your cap table, and the data room shows them as charts and tables.' },
					{ title: 'Choose what each recipient can see', body: 'Each link shows the categories its circle can access. Circles for investors, accountants, legal advisers and lenders are included.' }
				]
			},
			investor: {
				title: 'For investors',
				items: [
					{ title: 'Open it with a link and a code', body: 'No account. Enter the six-digit code and accept the confidentiality notice.' },
					{ title: 'Always the current records', body: 'The link reads the data room each time it opens, so new metrics and documents show up right away.' },
					{ title: 'More as diligence moves on', body: 'Widen the circle as diligence moves on, and the same link shows more categories, up to what the person who made the link can see.' }
				]
			},
			scope: 'What each circle can see',
			guard: 'Links last at most 7 days, and downloading originals is a separate choice. Every page of a downloaded PDF is stamped with the link and the time.'
		},
		ownership: {
			title: 'Your company keeps its data',
			lead: 'Conversations and working files live on your company computer from the start. How much more you run yourself is up to you.',
			table: {
				caption: 'What lives where',
				what: 'What',
				hosted: 'With intern.kim',
				selfHosted: 'Self-hosted',
				rows: [
					{ what: 'AI coworker, conversations, working files', hosted: 'Your computer', selfHosted: 'Your computer' },
					{ what: 'Company records and file storage', hosted: 'intern.kim', selfHosted: 'Your Supabase' },
					{ what: 'Web app and sign-in address', hosted: 'intern.kim', selfHosted: 'Your domain' }
				],
				note: 'Messages are never stored in the company record on intern.kim. Kim mini recovery snapshots are encrypted to your company key.'
			},
			items: [
				{ title: 'Run it yourself if we ever stop', body: "Intern Kim's source code is open under Apache-2.0. If Yeomyeonggeori stops the service, your company can run it and keep going." },
				{ title: 'Audit and patch it yourself', body: 'Anyone can read the code. Your team or a security auditor can verify it, fix what they find and deploy the fix.' },
				{ title: 'Works as the person who asked', body: "Each member has their own account on the company computer, and Intern Kim works under the account and permissions of whoever asked. What they can't open, it can't either." },
				{ title: 'Asks before it sends, deletes or changes rules', body: 'Intern Kim waits for your confirmation before sending email, approving leave, deleting records or changing company policies. Recurring work is confirmed once, when you set it up.' }
			],
			source: 'Read the code on GitHub'
		},
		plans: {
			title: 'An in-house AI coworker without the big bill',
			lead: 'One computer at your company handles requests from the whole team. There are no servers to rent.',
			pay: {
				title: 'Seats and AI usage',
				items: [
					{ title: 'Per seat', body: 'Pay per member. AI credits are included, and you can buy more when you run low.' },
					{ title: 'Pay for what you use', body: 'Each AI call takes what it used from your credits. Light decisions go to a dedicated decision model.' },
					{ title: 'Your OpenRouter key', body: 'Connect it and pay model costs directly.' }
				]
			},
			run: {
				title: 'A computer for your AI coworker',
				items: [
					{ title: 'Kim mini', body: 'A small computer that arrives with Intern Kim installed. Connect power and internet, then link it to your company.' },
					{ title: 'Several Kim minis', body: 'Recovery snapshots encrypted to your company key are saved every 15 minutes, so another Kim mini can take over if one stops.' },
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
