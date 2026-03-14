import "dotenv/config";
import { fetchMarketOverview } from "./fetchData.js";
import { fetchNews } from "./fetchNews.js";
import { generateBriefing } from "./summarizer.js";

async function main(): Promise<void> {
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
	console.log(briefing.response);
}

main().catch((error: unknown) => {
	const message = error instanceof Error ? error.message : String(error);
	console.error(`Error: ${message}`);
	process.exitCode = 1;
});
