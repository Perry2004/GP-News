import axios from "axios";
import YahooFinance from "yahoo-finance2";
import { z } from "zod";

interface YahooFinanceInstance {
	quote(symbol: string): Promise<unknown>;
}

const yahooFinance = new (
	YahooFinance as unknown as {
		new (options?: { suppressNotices?: string[] }): YahooFinanceInstance;
	}
)({
	suppressNotices: ["yahooSurvey"],
});

const envSchema = z.object({
	TWELVE_DATA_API_KEY: z.string().min(1),
});

const TWELVE_DATA_BASE_URL = "https://api.twelvedata.com";

const numericString = z
	.union([z.string(), z.number()])
	.transform((value) => String(value));
const textLike = z
	.union([z.string(), z.number()])
	.transform((value) => String(value));
const optionalNumericString = z.preprocess(
	(value) => (value == null ? undefined : value),
	numericString.optional(),
);
const optionalTextLike = z.preprocess(
	(value) => (value == null ? undefined : value),
	textLike.optional(),
);

const quoteSchema = z.object({
	symbol: textLike,
	name: optionalTextLike,
	exchange: optionalTextLike,
	close: numericString,
	previous_close: optionalNumericString,
	change: optionalNumericString,
	percent_change: optionalNumericString,
	currency: optionalTextLike,
	timestamp: optionalTextLike,
	volume: optionalNumericString,
});

const errorSchema = z.object({
	code: z.number().optional(),
	message: z.string(),
	status: z.string().optional(),
});

const indexTargets = [
	{ label: "NASDAQ", symbol: "^IXIC" },
	{ label: "DAX", symbol: "^GDAXI" },
	{ label: "S&P 500", symbol: "^GSPC" },
	{ label: "Dow Jones", symbol: "^DJI" },
	{ label: "Nikkei", symbol: "^N225" },
] as const;

const fxTargets = [
	{ label: "USD/JPY", symbol: "USD/JPY" },
	{ label: "USD/CNY", symbol: "USD/CNY" },
	{ label: "JPY/CNY", symbol: "JPY/CNY" },
] as const;

type Quote = z.infer<typeof quoteSchema>;

async function fetchQuote(symbol: string): Promise<Quote> {
	const envVars = envSchema.parse(process.env);
	const apiKey = envVars.TWELVE_DATA_API_KEY;

	const response = await axios.get(`${TWELVE_DATA_BASE_URL}/quote`, {
		params: {
			symbol,
			apikey: apiKey,
		},
		timeout: 15_000,
	});

	const errorParsed = errorSchema.safeParse(response.data);
	if (errorParsed.success) {
		throw new Error(
			`Twelve Data error for ${symbol}: ${errorParsed.data.message}`,
		);
	}

	const parsed = quoteSchema.safeParse(response.data);
	if (!parsed.success) {
		throw new Error(
			`Invalid quote payload for ${symbol}: ${parsed.error.issues
				.map((issue) => issue.message)
				.join(", ")}`,
		);
	}

	return parsed.data;
}

type YahooQuoteResponse = {
	symbol: string;
	shortName?: string;
	longName?: string;
	fullExchangeName?: string;
	regularMarketPrice: number;
	regularMarketPreviousClose?: number;
	regularMarketChange?: number;
	regularMarketChangePercent?: number;
	currency?: string;
	regularMarketTime?: number | string | Date;
	regularMarketVolume?: number;
};

async function fetchYahooQuote(symbol: string): Promise<Quote> {
	const result = (await yahooFinance.quote(symbol)) as YahooQuoteResponse;

	if (!result) {
		throw new Error(`Yahoo Finance error for ${symbol}: No data returned`);
	}

	return {
		symbol: result.symbol,
		name: result.shortName ?? result.longName ?? undefined,
		exchange: result.fullExchangeName ?? undefined,
		close: String(result.regularMarketPrice),
		previous_close: result.regularMarketPreviousClose
			? String(result.regularMarketPreviousClose)
			: undefined,
		change: result.regularMarketChange
			? String(result.regularMarketChange)
			: undefined,
		percent_change: result.regularMarketChangePercent
			? String(result.regularMarketChangePercent)
			: undefined,
		currency: result.currency ?? undefined,
		timestamp: result.regularMarketTime
			? new Date(result.regularMarketTime).toISOString()
			: undefined,
		volume: result.regularMarketVolume
			? String(result.regularMarketVolume)
			: undefined,
	};
}

export async function fetchMarketOverview(): Promise<{
	timestamp: string;
	indices: Array<{ label: string; quote: Quote }>;
	fx: Array<{ label: string; quote: Quote }>;
}> {
	const indexQuotes = await Promise.all(
		indexTargets.map(async ({ label, symbol }) => ({
			label,
			quote: await fetchYahooQuote(symbol),
		})),
	);

	const fxQuotes = await Promise.all(
		fxTargets.map(async ({ label, symbol }) => ({
			label,
			quote: await fetchQuote(symbol),
		})),
	);

	const timestamp =
		indexQuotes[0]?.quote.timestamp ??
		fxQuotes[0]?.quote.timestamp ??
		new Date().toISOString();

	return {
		timestamp,
		indices: indexQuotes,
		fx: fxQuotes,
	};
}
