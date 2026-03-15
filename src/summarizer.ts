import { ChatGoogleGenerativeAI } from "@langchain/google-genai";
import "dotenv/config";
import z from "zod";
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

const briefingResponseSchema = z.object({
	content: z.string().min(1).describe("Email briefing response"),
	subject: z.string().min(1).describe("Email subject line"),
});

type BriefingResponse = z.infer<typeof briefingResponseSchema>;

function buildBriefingPrompt(data: BriefingData): string {
	return `
        You are preparing GP News Daily Briefing for an executive audience.
        Generate one email briefing that can be read in about 3 minutes.
        High-signal, low-noise, concise, no filler.

        Output format:
        1) Email Priority: NORMAL or HIGH with a one-line reason.
        2) Market Snapshot: NASDAQ, DAX, S&P 500, Dow Jones, Nikkei, and JPY/CNY/USD with key move and likely driver in plain English.
        3) Priority-Ranked News: numbered list of most important developments, each 1-2 sentences, merged and deduplicated.
        4) Tone Differences: only include if meaningful framing differences exist across outlets.
        5) Tech Tendency: compact trend + implication (AI models/infrastructure, chips, hardware, Big Tech strategy).
        6) Polymarket Watch: top 3 relevant contracts across politics, technology, war. If contract prices are unavailable, infer directional market pricing from the input and label it as inferred.

        Source policy and weighting:
        - Reuters/AP for baseline facts
        - Bloomberg/WSJ/FT for market interpretation
        - NHK/Nikkei Asia for Japan/Asia framing
        - Al Jazeera for Middle East framing (translate Arabic framing naturally to English if present)
        - Cross-check major claims when possible
        - Avoid redundant sourcing unless interpretation differs materially

        Email HIGH priority only if at least one item is immediately market-moving, major military escalation, major policy decision, or materially shifts global risk sentiment.

		ENSURE THE BRIEFING CONTENT IS FORMATTED PROPERLY WITH MARKDOWN FOR EASY READING IN EMAIL. USE HEADERS, BOLD, AND BULLETS AS APPROPRIATE.

        News + market input JSON:
        ${JSON.stringify(data)}
    `;
}

export async function generateBriefing(
	data: BriefingData,
): Promise<BriefingResponse> {
	const prompt = buildBriefingPrompt(data);
	const modelName = envVars.MODEL_NAME;
	const model = new ChatGoogleGenerativeAI({
		model: modelName,
		apiKey: envVars.AI_API_KEY,
	});
	const structuredModel = model.withStructuredOutput(briefingResponseSchema);
	const response = await structuredModel.invoke(prompt);
	return response;
}
