import axios from "axios";
import * as dotenv from "dotenv";
import { z } from "zod";

dotenv.config();

const envVars = z
	.object({
		TWELVE_DATA_API_KEY: z.string().min(1),
	})
	.parse(process.env);

const TWELVE_DATA_BASE_URL = "https://api.twelvedata.com";
const apiKey = envVars.TWELVE_DATA_API_KEY;

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

// symbols: https://support.twelvedata.com/en/articles/5620513-how-to-find-all-available-symbols-at-twelve-data
const indexTargets = [
	{ label: "NASDAQ", symbol: "QQQ" },
	{ label: "DAX", symbol: "EWG" },
	{ label: "S&P 500", symbol: "SPY" },
	{ label: "Dow Jones", symbol: "DIA" },
	{ label: "Nikkei", symbol: "EWJ" },
] as const;

const fxTargets = [
	{ label: "USD/JPY", symbol: "USD/JPY" },
	{ label: "USD/CNY", symbol: "USD/CNY" },
	{ label: "JPY/CNY", symbol: "JPY/CNY" },
] as const;

type Quote = z.infer<typeof quoteSchema>;

async function fetchQuote(symbol: string): Promise<Quote> {
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

export async function fetchMarketOverview(): Promise<{
	timestamp: string;
	indices: Array<{ label: string; quote: Quote }>;
	fx: Array<{ label: string; quote: Quote }>;
}> {
	const indexQuotes = await Promise.all(
		indexTargets.map(async ({ label, symbol }) => ({
			label,
			quote: await fetchQuote(symbol),
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
