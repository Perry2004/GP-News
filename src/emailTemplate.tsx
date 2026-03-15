import {
	Body,
	Container,
	Head,
	Heading,
	Hr,
	Html,
	Link,
	Preview,
	Section,
	Tailwind,
	Text,
} from "@react-email/components";
import type { BriefingEmailData } from "./briefingSchema.js";

type GpNewsEmailProps = {
	briefing: BriefingEmailData;
};

function buildPriorityBadgeClasses(
	level: BriefingEmailData["priority"]["level"],
): {
	background: string;
	text: string;
} {
	if (level === "HIGH") {
		return {
			background: "bg-red-100",
			text: "text-red-800",
		};
	}

	return {
		background: "bg-emerald-100",
		text: "text-emerald-800",
	};
}

export function GpNewsEmailTemplate({ briefing }: GpNewsEmailProps) {
	const badgeClasses = buildPriorityBadgeClasses(briefing.priority.level);

	return (
		<Html>
			<Head />
			<Preview>
				{briefing.subject} - {briefing.priority.level} priority update
			</Preview>
			<Tailwind>
				<Body className="m-0 bg-slate-100 px-3 py-8 font-sans text-slate-900">
					<Container className="mx-auto max-w-3xl overflow-hidden rounded-xl border border-slate-200 bg-white shadow-sm">
						<Section className="bg-slate-900 px-8 py-8 text-white">
							<Text className="m-0 text-xs uppercase tracking-[0.14em] text-slate-300">
								GP News Intelligence Desk
							</Text>
							<Heading className="mb-0 mt-3 text-3xl leading-tight font-bold text-white">
								{briefing.title}
							</Heading>
							<Text className="mt-5 mb-0 text-sm leading-6 text-slate-200">
								Daily executive synthesis of market risk, geopolitical pressure,
								and technology trend shifts.
							</Text>
						</Section>

						<Section className="px-8 pt-7 pb-2">
							<Text
								className={`m-0 inline-block rounded-full px-3 py-1 text-xs font-bold tracking-wide uppercase ${badgeClasses.background} ${badgeClasses.text}`}
							>
								Email Priority: {briefing.priority.level}
							</Text>
							<Text className="mt-3 mb-0 text-sm leading-6 text-slate-700">
								{briefing.priority.reason}
							</Text>
						</Section>

						<Section className="px-8 py-2">
							<Heading className="mb-2 text-xl font-semibold text-slate-900">
								Market Snapshot
							</Heading>
							{briefing.marketSnapshot.map((item) => (
								<Section
									key={`${item.name}-${item.value}`}
									className="mb-3 rounded-lg border border-slate-200 bg-slate-50 px-4 py-3"
								>
									<Text className="m-0 text-sm font-semibold text-slate-900">
										{item.name}: {item.value} ({item.move})
									</Text>
									<Text className="mt-1 mb-0 text-sm leading-6 text-slate-700">
										{item.driver}
									</Text>
								</Section>
							))}
						</Section>

						<Hr className="my-2 border-slate-200" />

						<Section className="px-8 py-2">
							<Heading className="mb-2 text-xl font-semibold text-slate-900">
								Priority-Ranked News
							</Heading>
							{briefing.priorityNews.map((item, index) => (
								<Section
									key={`${item.headline}-${item.summary.slice(0, 32)}`}
									className="mb-4"
								>
									<Text className="m-0 text-sm font-bold text-slate-900">
										{index + 1}. {item.headline}
									</Text>
									<Text className="mt-1 mb-0 text-sm leading-6 text-slate-700">
										{item.summary}
									</Text>
									{item.whyItMatters ? (
										<Text className="mt-1 mb-0 text-xs leading-5 font-semibold uppercase tracking-wide text-slate-500">
											Why It Matters: {item.whyItMatters}
										</Text>
									) : null}
									{item.link ? (
										<Link
											href={item.link}
											className="mt-1 block text-xs text-slate-400 underline"
										>
											Read source →
										</Link>
									) : null}
								</Section>
							))}
						</Section>

						<Hr className="my-2 border-slate-200" />

						<Section className="px-8 py-2">
							<Heading className="mb-2 text-xl font-semibold text-slate-900">
								Tech Tendency
							</Heading>
							<Text className="m-0 text-base font-semibold text-slate-900">
								{briefing.techTendency.theme}
							</Text>
							<Text className="mt-1 mb-0 text-sm leading-6 text-slate-700">
								Signal: {briefing.techTendency.signal}
							</Text>
							<Text className="mt-1 mb-0 text-sm leading-6 text-slate-700">
								Implication: {briefing.techTendency.implication}
							</Text>
							{briefing.techTendency.link ? (
								<Link
									href={briefing.techTendency.link}
									className="mt-1 block text-xs text-slate-400 underline"
								>
									Read source →
								</Link>
							) : null}
						</Section>

						<Section className="mt-4 bg-slate-50 px-8 py-5">
							<Text className="m-0 text-xs leading-5 text-slate-500">
								Prepared automatically from cross-source market and news
								signals. Verify critical operational decisions against primary
								reporting before execution.
							</Text>
						</Section>
					</Container>
				</Body>
			</Tailwind>
		</Html>
	);
}

export function renderBriefingAsText(briefing: BriefingEmailData): string {
	const lines: string[] = [];

	lines.push(briefing.title);
	lines.push("");
	lines.push(`Email Priority: ${briefing.priority.level}`);
	lines.push(briefing.priority.reason);
	lines.push("");
	lines.push("Market Snapshot");
	for (const item of briefing.marketSnapshot) {
		lines.push(`- ${item.name}: ${item.value} (${item.move})`);
		lines.push(`  ${item.driver}`);
	}

	lines.push("");
	lines.push("Priority-Ranked News");
	for (const [index, item] of briefing.priorityNews.entries()) {
		lines.push(`${index + 1}. ${item.headline}`);
		lines.push(`   ${item.summary}`);
		if (item.whyItMatters) {
			lines.push(`   Why It Matters: ${item.whyItMatters}`);
		}
		if (item.link) {
			lines.push(`   ${item.link}`);
		}
	}

	lines.push("");
	lines.push("Tech Tendency");
	lines.push(`${briefing.techTendency.theme}`);
	lines.push(`Signal: ${briefing.techTendency.signal}`);
	lines.push(`Implication: ${briefing.techTendency.implication}`);
	if (briefing.techTendency.link) {
		lines.push(`   ${briefing.techTendency.link}`);
	}

	return lines.join("\n");
}
