import axios from "axios";
import { z } from "zod";

const envVars = z
	.object({
		NEWS_DATA_API_KEY: z.string().min(1),
	})
	.parse(process.env);

const NEWSDATA_BASE_URL = "https://newsdata.io/api/1/latest";
const accessKey = envVars.NEWS_DATA_API_KEY;
const DEFAULT_TOTAL_LIMIT = 50;
const NEWSDATA_MAX_SIZE = 10;

const SOURCE_CONFIGS = [
	"bloomberg.com",
	"reuters.com",
	"wsj.com",
	"ft.com",
	"apnews.com",
	"nhk.or.jp",
	"xtech.nikkei.com",
	"aljazeera.net",
] as const;

type SourceConfig = (typeof SOURCE_CONFIGS)[number];
const SOURCE_BATCHES = [
	SOURCE_CONFIGS.slice(0, 4),
	SOURCE_CONFIGS.slice(4),
] as const;

const newsArticleSchema = z.object({
	article_id: z.string().min(1).optional(),
	title: z.string().min(1),
	link: z.string().min(1),
	creator: z.array(z.string()).nullable().optional(),
	description: z.string().nullable().optional(),
	content: z.string().nullable().optional(),
	pubDate: z.string(),
	image_url: z.string().nullable().optional(),
	source_id: z.string().nullable().optional(),
	source_name: z.string().nullable().optional(),
	source_url: z.string().nullable().optional(),
	language: z.string().nullable().optional(),
	country: z.array(z.string()).optional(),
	category: z.array(z.string()).optional(),
});

const newsResponseSchema = z.object({
	status: z.literal("success"),
	totalResults: z.number().optional(),
	nextPage: z.string().optional(),
	results: z.array(newsArticleSchema),
});

const errorResponseSchema = z.object({
	status: z.literal("error"),
	results: z.object({
		code: z.string().optional(),
		message: z.string(),
	}),
});

export type NewsArticle = z.infer<typeof newsArticleSchema>;

async function fetchBatchArticles(
	batch: readonly SourceConfig[],
	size: number,
	page?: string,
): Promise<{ articles: NewsArticle[]; nextPage?: string }> {
	const response = await axios.get(NEWSDATA_BASE_URL, {
		params: {
			apikey: accessKey,
			domainurl: batch.join(","),
			removeduplicate: 1,
			size,
			...(page ? { page } : {}),
		},
		timeout: 15_000,
	});

	const errorParsed = errorResponseSchema.safeParse(response.data);
	if (errorParsed.success) {
		throw new Error(`NewsData.io error: ${errorParsed.data.results.message}`);
	}

	const parsed = newsResponseSchema.safeParse(response.data);
	if (!parsed.success) {
		throw new Error(
			`Invalid news payload for batch ${batch.join(", ")}: ${parsed.error.issues
				.map((issue) => issue.message)
				.join(", ")}`,
		);
	}
	if (parsed.data.nextPage) {
		return {
			articles: parsed.data.results,
			nextPage: parsed.data.nextPage,
		};
	}

	return {
		articles: parsed.data.results,
	};
}

export async function fetchNews(limit = DEFAULT_TOTAL_LIMIT): Promise<{
	timestamp: string;
	total: number;
	articles: NewsArticle[];
}> {
	const targetTotal = Math.max(1, limit);
	const cursors = SOURCE_BATCHES.map(() => undefined as string | undefined);
	const exhausted = SOURCE_BATCHES.map(() => false);
	const seen = new Set<string>();
	const collected: NewsArticle[] = [];

	while (collected.length < targetTotal) {
		const beforeRound = collected.length;
		let roundCount = 0;

		for (let index = 0; index < SOURCE_BATCHES.length; index++) {
			if (roundCount >= NEWSDATA_MAX_SIZE || collected.length >= targetTotal) {
				break;
			}
			if (exhausted[index]) {
				continue;
			}
			const batch = SOURCE_BATCHES[index];
			if (!batch) {
				continue;
			}

			const requestSize = Math.min(
				NEWSDATA_MAX_SIZE - roundCount,
				targetTotal - collected.length,
				NEWSDATA_MAX_SIZE,
			);
			const { articles, nextPage } = await fetchBatchArticles(
				batch,
				requestSize,
				cursors[index],
			);
			cursors[index] = nextPage;
			exhausted[index] = !nextPage;

			for (const article of articles) {
				const key = article.article_id?.trim() || article.link.trim();
				if (seen.has(key)) {
					continue;
				}
				seen.add(key);
				collected.push(article);
				roundCount += 1;
				if (
					roundCount >= NEWSDATA_MAX_SIZE ||
					collected.length >= targetTotal
				) {
					break;
				}
			}
		}

		if (collected.length === beforeRound) {
			break;
		}
	}

	return {
		timestamp: new Date().toISOString(),
		total: collected.length,
		articles: collected,
	};
}
