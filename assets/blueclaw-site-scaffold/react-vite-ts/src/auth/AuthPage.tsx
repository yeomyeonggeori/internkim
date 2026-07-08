import { useState } from "react";
import { Button } from "../components/ui/button";
import { Input } from "../components/ui/input";
import { Label } from "../components/ui/label";
import type { SiteAuth } from "../site-content";
import { signIn, signUp } from "./session";

type AuthPageProps = {
	mode: "login" | "signup";
	auth: SiteAuth;
};

export function AuthPage({ mode, auth }: AuthPageProps) {
	const [email, setEmail] = useState("");
	const [name, setName] = useState("");
	const [password, setPassword] = useState("");
	const [message, setMessage] = useState("");
	const [isBusy, setIsBusy] = useState(false);
	const isSignup = mode === "signup";

	const submit = async (event: React.FormEvent) => {
		event.preventDefault();
		setIsBusy(true);
		setMessage("");
		const result = isSignup
			? await signUp(auth.userCollection, email, password, name)
			: await signIn(auth.userCollection, email, password);
		setIsBusy(false);
		if (!result.ok) {
			setMessage(result.message);
			return;
		}
		window.location.hash = auth.redirectAfterLogin;
	};

	return (
		<section className="mx-auto flex min-h-[60vh] max-w-sm flex-col justify-center py-20">
			<h1 className="text-3xl font-bold tracking-tight">{isSignup ? "회원가입" : "로그인"}</h1>
			<form onSubmit={submit} className="mt-8 flex flex-col gap-4">
				{isSignup ? (
					<div className="flex flex-col gap-1.5">
						<Label htmlFor="auth-name">이름</Label>
						<Input id="auth-name" value={name} onChange={(event) => setName(event.target.value)} required />
					</div>
				) : null}
				<div className="flex flex-col gap-1.5">
					<Label htmlFor="auth-email">이메일</Label>
					<Input id="auth-email" type="email" value={email} onChange={(event) => setEmail(event.target.value)} required />
				</div>
				<div className="flex flex-col gap-1.5">
					<Label htmlFor="auth-password">비밀번호</Label>
					<Input id="auth-password" type="password" minLength={8} value={password} onChange={(event) => setPassword(event.target.value)} required />
				</div>
				{message ? <p className="text-sm text-destructive">{message}</p> : null}
				<Button type="submit" disabled={isBusy} className="mt-2">
					{isBusy ? "처리 중..." : isSignup ? "가입하기" : "로그인"}
				</Button>
			</form>
			<p className="mt-6 text-sm text-muted-foreground">
				{isSignup ? (
					<>이미 계정이 있나요? <a className="font-medium underline underline-offset-4" href={"#" + auth.loginPath}>로그인</a></>
				) : auth.allowSignup ? (
					<>계정이 없나요? <a className="font-medium underline underline-offset-4" href={"#" + auth.signupPath}>회원가입</a></>
				) : null}
			</p>
		</section>
	);
}
