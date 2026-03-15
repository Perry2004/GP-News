# GP-News 📬

GP-News is a daily news briefing automation tool that fetches market data and global news, generates a high-signal executive summary using AI, and delivers it via email.

## Features ✨

- **Market Overview**: Fetches key equity indices and FX rates using `yahoo-finance2` and [Twelve Data](https://twelvedata.com).
- **Global News Aggregation**: Collects latest news from top-tier sources (Bloomberg, Reuters, WSJ, FT, AP, NHK, Nikkei, Al Jazeera) via `newsdata.io`.
- **Content Extraction**: Uses `defuddle` and `jsdom` to extract content from news articles.
- **AI-Powered Summarization**: Leverages LLM to generate professional, concise briefings.
- **Email Delivery**: Sends formatted HTML emails using `react-email` and AWS SES.
- **Serverless Ready**: Includes a Lambda handler for easy deployment to AWS.

## Prerequisites 🛠️

- [Node.js](https://nodejs.org/)
- [pnpm](https://pnpm.io/)
- [AWS Account](https://aws.amazon.com/) (for SES)
- [NewsData.io API Key](https://newsdata.io/)
- [Google Gemini API Key](https://aistudio.google.com/) (can be changed to another LLM provider if needed)

## Project Structure 📂

- `src/main.ts`: Entry point for local execution.
- `src/lambda-handler.ts`: Entry point for AWS Lambda.
- `src/fetchData.ts`: Market data fetching logic.
- `src/fetchNews.ts`: News aggregation and extraction.
- `src/summarizer.ts`: AI summary generation logic.
- `src/emailer.ts` & `src/emailTemplate.tsx`: Email rendering and SES integration.
- `src/briefingSchema.ts`: Zod schema for structured AI output.

## Tech Stack 💻

- **Runtime**: Node.js, TypeScript
- **Tooling**: Biome (Linting/Formatting), tsx, Husky
- **Libraries**: LangChain Core, React Email, AWS SDK v3, Zod, Axios, Yahoo Finance 2, Defuddle, JSDOM
