import axios from "axios";
import { Defuddle } from "defuddle/node";
import { z } from "zod";

const envVars = z
	.object({
		MEDIA_STACK_API_KEY: z.string().min(1),
	})
	.parse(process.env);

const MEDIASTACK_BASE_URL = "http://api.mediastack.com/v1";
const accessKey = envVars.MEDIA_STACK_API_KEY;

// exclude sources that need paid subscription
const BLOCKED_SOURCES = ["nytimes", "financial-times", "fox-news"]
	.map((source) => `-${source}`)
	.join(",");

// Mediastack does not have a "politics" category; "general" is the closest match
const TARGET_CATEGORIES = "general,business,science,technology";

const newsArticleSchema = z.object({
	author: z.string().nullable().optional(),
	title: z.string(),
	description: z.string().nullable().optional(),
	url: z.string().url(),
	source: z.string().nullable().optional(),
	image: z.string().nullable().optional(),
	category: z.string().optional(),
	language: z.string().optional(),
	country: z.string().nullable().optional(),
	published_at: z.string(),
});

const newsResponseSchema = z.object({
	pagination: z.object({
		limit: z.number(),
		offset: z.number(),
		count: z.number(),
		total: z.number(),
	}),
	data: z.array(newsArticleSchema),
});

const errorResponseSchema = z.object({
	error: z.object({
		code: z.string().optional(),
		message: z.string(),
		context: z.record(z.string(), z.unknown()).optional(),
	}),
});

export type NewsArticle = z.infer<typeof newsArticleSchema>;
export type EnrichedNewsArticle = NewsArticle & {
	extractedTitle: string | null;
	extractedContent: string | null;
	extractedWordCount: number | null;
	extractionError?: string;
};

async function extractNewsContent(url: string): Promise<{
	extractedTitle: string | null;
	extractedContent: string | null;
	extractedWordCount: number | null;
	extractionError?: string;
}> {
	try {
		const pageResponse = await axios.get<string>(url, {
			responseType: "text",
			timeout: 15_000,
			headers: {
				"User-Agent": "Mozilla/5.0 (compatible; GP-News/1.0)",
			},
		});

		const parsed = await Defuddle(pageResponse.data, url, {
			markdown: true,
		});

		return {
			extractedTitle: parsed.title || null,
			extractedContent: parsed.content?.trim() || null,
			extractedWordCount: parsed.wordCount ?? null,
		};
	} catch (error: unknown) {
		const message = error instanceof Error ? error.message : String(error);
		return {
			extractedTitle: null,
			extractedContent: null,
			extractedWordCount: null,
			extractionError: message,
		};
	}
}

export async function fetchTopNews(limit = 5): Promise<{
	timestamp: string;
	total: number;
	articles: EnrichedNewsArticle[];
}> {
	const response = await axios.get(`${MEDIASTACK_BASE_URL}/news`, {
		params: {
			access_key: accessKey,
			sources: BLOCKED_SOURCES,
			categories: TARGET_CATEGORIES,
			languages: "en",
			sort: "popularity",
			limit,
		},
		timeout: 15_000,
	});

	const errorParsed = errorResponseSchema.safeParse(response.data);
	if (errorParsed.success) {
		throw new Error(`Mediastack error: ${errorParsed.data.error.message}`);
	}

	const parsed = newsResponseSchema.safeParse(response.data);
	if (!parsed.success) {
		throw new Error(
			`Invalid news response payload: ${parsed.error.issues
				.map((issue) => issue.message)
				.join(", ")}`,
		);
	}

	const enrichedArticles = await Promise.all(
		parsed.data.data.map(async (article) => ({
			...article,
			...(await extractNewsContent(article.url)),
		})),
	);

	return {
		timestamp: new Date().toISOString(),
		total: parsed.data.pagination.total,
		articles: enrichedArticles,
	};
}
