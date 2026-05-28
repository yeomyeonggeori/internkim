import { ArrowRight, CheckCircle2, LayoutDashboard, Palette, Sparkles, Workflow } from "lucide-react";
import { Badge } from "./components/ui/badge";
import { Button } from "./components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "./components/ui/card";

const workflowItems = [
	{ icon: Palette, title: "Stitch DESIGN.md", copy: "Canonical tokens come before UI code." },
	{ icon: LayoutDashboard, title: "shadcn primitives", copy: "Use real controls instead of decorative placeholders." },
	{ icon: Workflow, title: "Usable first screen", copy: "Open with the requested workflow." },
	{ icon: CheckCircle2, title: "Visual QA", copy: "Preview desktop and mobile before publish." },
];

function App() {
	return (
		<main className="min-h-screen bg-background text-foreground">
			<section className="mx-auto grid min-h-screen w-full max-w-7xl gap-8 px-6 py-8 md:grid-cols-[1fr_420px] md:px-10">
				<div className="flex flex-col justify-between gap-10 py-8">
					<div className="flex items-center gap-3">
						<div className="flex h-10 w-10 items-center justify-center rounded-lg bg-primary text-primary-foreground">
							<Sparkles className="h-5 w-5" />
						</div>
						<p className="text-sm font-semibold text-muted-foreground">InternKim React prototype</p>
					</div>
					<div className="max-w-3xl space-y-7">
						<Badge variant="secondary" className="w-fit">Beautiful default scaffold</Badge>
						<h1 className="text-4xl font-semibold tracking-[0px] md:text-6xl">Replace this with the requested product workflow.</h1>
						<p className="text-xl leading-8 text-muted-foreground">
							Keep DESIGN.md as the visual source of truth and ship a prototype that looks composed before it is published.
						</p>
						<Button>
							Start from the workflow
							<ArrowRight className="h-4 w-4" />
						</Button>
					</div>
				</div>
				<Card className="self-center">
					<CardHeader>
						<CardDescription>Generation loop</CardDescription>
						<CardTitle>Design before code</CardTitle>
					</CardHeader>
					<CardContent className="space-y-4">
						{workflowItems.map((item) => (
							<div key={item.title} className="flex gap-4 rounded-lg border bg-card p-4">
								<div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-md bg-secondary text-secondary-foreground">
									<item.icon className="h-5 w-5" />
								</div>
								<div>
									<h2 className="font-semibold">{item.title}</h2>
									<p className="mt-1 text-sm leading-6 text-muted-foreground">{item.copy}</p>
								</div>
							</div>
						))}
					</CardContent>
				</Card>
			</section>
		</main>
	);
}

export default App;
