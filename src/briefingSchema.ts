import z from "zod";

const priorityLevelSchema = z.enum(["NORMAL", "HIGH"]);

export const briefingSchema = z.object({
	subject: z
		.string()
		.min(1)
		.describe("Professional, concise subject line for the briefing email."),
	title: z
		.string()
		.min(1)
		.describe("Main report title shown at the top of the email."),
	priority: z.object({
		level: priorityLevelSchema.describe(
			"Priority level. Use HIGH only for immediate market-moving risk.",
		),
		reason: z
			.string()
			.min(1)
			.describe("One sentence explaining the chosen priority level."),
	}),
	marketSnapshot: z
		.array(
			z.object({
				name: z.string().min(1).describe("Instrument or market name."),
				value: z
					.string()
					.min(1)
					.describe(
						"Current level or quoted value, as a display-ready string.",
					),
				move: z
					.string()
					.min(1)
					.describe("Move direction and size, e.g. -0.59% or +1.12%"),
				driver: z
					.string()
					.min(1)
					.describe("Likely primary driver in plain executive English."),
			}),
		)
		.min(1)
		.describe("Cross-asset market bullets, usually indices and key FX pairs."),
	priorityNews: z
		.array(
			z.object({
				headline: z.string().min(1).describe("Ranked news headline."),
				summary: z
					.string()
					.min(1)
					.describe(
						"1-2 sentence factual summary, deduplicated across sources.",
					),
				whyItMatters: z
					.string()
					.min(1)
					.optional()
					.describe(
						"Optional one-line market relevance or strategic implication.",
					),
				link: z
					.url()
					.optional()
					.describe(
						"Source URL for the primary article backing this news item.",
					),
			}),
		)
		.min(1)
		.describe("Most important developments, ranked by urgency and impact."),
	techTendency: z.object({
		theme: z.string().min(1).describe("Name of the technology trend."),
		signal: z.string().min(1).describe("Key signal observed today."),
		implication: z
			.string()
			.min(1)
			.describe("Forward-looking implication for strategy or markets."),
		link: z
			.url()
			.optional()
			.describe(
				"Source URL for the primary article backing the tech tendency.",
			),
	}),
});

export type BriefingEmailData = z.infer<typeof briefingSchema>;
