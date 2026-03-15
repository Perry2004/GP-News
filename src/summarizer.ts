import { ChatGoogleGenerativeAI } from "@langchain/google-genai";
import "dotenv/config";
import z from "zod";
import { type BriefingEmailData, briefingSchema } from "./briefingSchema.js";
import type { fetchMarketOverview } from "./fetchData.js";
import type { AggregatedNewsArticle } from "./fetchNews.js";

const envVars = z
	.object({
		MODEL_NAME: z.string().min(1),
		AI_API_KEY: z.string().min(1),
	})
	.parse(process.env);

type BriefingData = {
	market: Awaited<ReturnType<typeof fetchMarketOverview>>;
	news: {
		timestamp: string;
		total: number;
		aggregatedNews: AggregatedNewsArticle[];
	};
};

function buildBriefingPrompt(data: BriefingData): string {
	return `
		You are preparing GP News Daily Briefing for an executive audience.
		Generate concise, factual, high-signal output that can be read in about 3 minutes.

		You must return JSON that strictly matches the provided schema.
		Do not include markdown, prose outside fields, or extra keys.

		Editorial rules:
		- Prioritize verified facts and avoid speculation.
		- Deduplicate overlapping stories and keep summaries compact.
		- Use professional language suitable for senior decision-makers.
		- Mark priority HIGH only for immediate market-moving risk, major military escalation, major policy decisions, or material global sentiment shifts.

		Section requirements:
		1) subject: Specific, concise, and professional.
		2) title: Usually "GP News Daily Briefing" unless a clearer headline is warranted.
		3) priority: level + one-line reason.
		4) marketSnapshot: Include key equity indices and major FX where available, with value, move, and plain-English driver.
		5) priorityNews: Ranked by urgency/impact. Each item needs headline and 1-2 sentence summary. whyItMatters is optional but preferred when impact is non-obvious. link should be the source article URL from the input data when available.
		6) techTendency: theme + current signal + implication. link should be the source article URL from the input data when available.

		Source weighting guidance:
		- Reuters/AP for baseline facts
		- Bloomberg/WSJ/FT for market interpretation
		- NHK/Nikkei Asia for Japan/Asia framing
		- Al Jazeera for Middle East framing
		- Cross-check major claims when possible
		- Avoid repeating similar sources unless framing materially differs

		News + market input JSON:
		${JSON.stringify(data)}
    `;
}

export async function generateBriefing(
	data: BriefingData,
): Promise<BriefingEmailData> {
	const prompt = buildBriefingPrompt(data);
	const modelName = envVars.MODEL_NAME;
	const model = new ChatGoogleGenerativeAI({
		model: modelName,
		apiKey: envVars.AI_API_KEY,
	});
	const structuredModel = model.withStructuredOutput(briefingSchema);
	const response = await structuredModel.invoke(prompt);
	return briefingSchema.parse(response);
}
