import { fetchMarketOverview } from "./fetchData.js";

async function main(): Promise<void> {
	const rawData = await fetchMarketOverview();
	console.log(JSON.stringify(rawData, null, 2));
}

main().catch((error: unknown) => {
	const message = error instanceof Error ? error.message : String(error);
	console.error(`Error: ${message}`);
	process.exitCode = 1;
});
