import "dotenv/config";
import { fileURLToPath } from "node:url";
import { sendBriefingEmail } from "./emailer.js";
import { fetchMarketOverview } from "./fetchData.js";
import { fetchNews } from "./fetchNews.js";
import { generateBriefing } from "./summarizer.js";

export async function runBriefing(): Promise<{ subject: string }> {
	const [marketData, newsData] = await Promise.all([
		fetchMarketOverview(),
		fetchNews(),
	]);
	console.log(
		`Data: ${JSON.stringify(
			{
				market: marketData,
				news: newsData,
			},
			null,
			2,
		)}`,
	);
	const briefing = await generateBriefing({
		market: marketData,
		news: newsData,
	});
	console.log(
		`Briefing generated: ${briefing.subject} [${briefing.priority.level}]`,
	);
	const sendResult = await sendBriefingEmail(briefing, briefing.subject);
	console.log(
		`Email sent via SES. MessageId: ${sendResult.map((res) => res.MessageId ?? "unknown").join(", ")}`,
	);
	return { subject: briefing.subject };
}

if (process.argv[1] === fileURLToPath(import.meta.url)) {
	runBriefing().catch((error: unknown) => {
		const message = error instanceof Error ? error.message : String(error);
		console.error(`Error: ${message}`);
		process.exitCode = 1;
	});
}
