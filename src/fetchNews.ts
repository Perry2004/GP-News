import axios from "axios";
import { Defuddle } from "defuddle/node";
import { JSDOM } from "jsdom";
import { z } from "zod";

const envSchema = z.object({
	NEWS_DATA_API_KEY: z.string().min(1),
});

const NEWSDATA_BASE_URL = "https://newsdata.io/api/1/latest";
const DEFAULT_TOTAL_LIMIT = 50;
const NEWSDATA_MAX_SIZE = 10;
const ARTICLE_FETCH_TIMEOUT_MS = 15_000;

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
type ExtractedNews = Awaited<ReturnType<typeof extractNewsContent>>;

export type AggregatedNewsArticle = Omit<NewsArticle, "title" | "content"> & {
	title: string;
	content: string | null;
};

function aggregateNewsArticle(
	news: NewsArticle,
	extracted: ExtractedNews,
): AggregatedNewsArticle {
	const newsDataContent =
		news.content?.trim() || news.description?.trim() || null;
	const hasExtractedContent = Boolean(
		extracted.extractedTitle || extracted.extractedContent,
	);

	if (!hasExtractedContent) {
		return {
			...news,
			title: news.title,
			content: newsDataContent,
		};
	}

	return {
		...news,
		title: extracted.extractedTitle?.trim() || news.title,
		content: extracted.extractedContent?.trim() || newsDataContent,
	};
}

async function fetchBatchArticles(
	batch: readonly SourceConfig[],
	size: number,
	page?: string,
): Promise<{ articles: NewsArticle[]; nextPage?: string }> {
	const envVars = envSchema.parse(process.env);
	const accessKey = envVars.NEWS_DATA_API_KEY;

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

async function extractNewsContent(url: string): Promise<{
	extractedTitle: string | null;
	extractedContent: string | null;
	extractedWordCount: number | null;
	extractionError?: string;
}> {
	try {
		const response = await axios.get(url, {
			timeout: ARTICLE_FETCH_TIMEOUT_MS,
			headers: {
				"User-Agent":
					"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36",
				Accept:
					"text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8",
				"Accept-Language": "en-US,en;q=0.9",
				Connection: "keep-alive",
			},
			maxRedirects: 5,
			responseType: "text",
		});

		const html = String(response.data ?? "");
		const dom = new JSDOM(html, { url });
		const pageTitle = dom.window.document
			.querySelector("meta[property='og:title']")
			?.getAttribute("content")
			?.trim();
		const fallbackTitle =
			pageTitle || dom.window.document.title?.trim() || null;
		const lowerHtml = html.toLowerCase();
		const blockedByCloudflare =
			lowerHtml.includes("attention required") &&
			lowerHtml.includes("cloudflare") &&
			lowerHtml.includes("you have been blocked");

		const parsed = await Defuddle(dom, url, {
			markdown: true,
		});
		const extractedContent = parsed.content?.trim() || null;
		const extractedWordCount = parsed.wordCount ?? null;
		const extractedTitle = parsed.title?.trim() || fallbackTitle;

		if (!extractedContent && blockedByCloudflare) {
			return {
				extractedTitle,
				extractedContent: null,
				extractedWordCount,
				extractionError: "Blocked by Cloudflare challenge page",
			};
		}

		return {
			extractedTitle,
			extractedContent,
			extractedWordCount,
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

export async function fetchNews(limit = DEFAULT_TOTAL_LIMIT): Promise<{
	timestamp: string;
	total: number;
	aggregatedNews: AggregatedNewsArticle[];
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

	const extractedNews = await Promise.all(
		collected.map(async (news) => extractNewsContent(news.link)),
	);
	const aggregatedNews = collected.map((news, index) =>
		aggregateNewsArticle(
			news,
			extractedNews[index] ?? {
				extractedTitle: null,
				extractedContent: null,
				extractedWordCount: null,
			},
		),
	);

	return {
		timestamp: new Date().toISOString(),
		total: collected.length,
		aggregatedNews,
	};
}
