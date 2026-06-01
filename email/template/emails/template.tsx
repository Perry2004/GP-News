import * as React from "react";
import {
  Body,
  Column,
  Container,
  Head,
  Heading,
  Hr,
  Html,
  Link,
  Preview,
  Row,
  Section,
  Text,
} from "react-email";

type PriorityLevel = "Low" | "Watch" | "Important" | "Critical";
type Confidence = "High" | "Medium" | "Low";

type MarketItem = {
  asset: string;
  level: string;
  dailyChange?: string;
  timestamp: string;
  driver: string;
  source: string;
};

type SourceItem = {
  label: string;
  url?: string;
};

type NewsCard = {
  topic: string;
  region: string;
  headline: string;
  summary: string;
  whyItMatters: string;
  sources: SourceItem[];
  priorityScore: number;
  confidence: Confidence;
  mustRead?: boolean;
};

type RegionalRadarItem = {
  region: string;
  sentence: string;
  sources: SourceItem[];
};

export type BriefingEmail = {
  subject: string;
  criticalityScore: number;
  priorityLevel: PriorityLevel;
  highPriorityTag: boolean;
  mainDriver: string;
  todaysSignal: string;
  readThisFirst: string[];
  marketSnapshot: {
    equityIndices: MarketItem[];
    fx: MarketItem[];
    ratesBonds: MarketItem[];
    commoditiesCryptoRisk: MarketItem[];
  };
  macroDataWatch: string[];
  policySignalWatch: string[];
  topNewsByTopic: {
    marketsMacro: NewsCard[];
    politicsPolicy: NewsCard[];
    warGeopoliticalRisk: NewsCard[];
    technologyAI: NewsCard[];
  };
  regionalRadar: RegionalRadarItem[];
  toneFramingDifferences?: string[];
  techTendency?: string[];
  polymarketWatch?: string[];
  watchNext: string[];
  whyThisMattersToday: string;
};

const sampleBriefing: BriefingEmail = {
  subject: "GP News Briefing - Morning - 2026-05-30 - GP Criticality 8.42",
  criticalityScore: 8.42,
  priorityLevel: "Critical",
  highPriorityTag: true,
  mainDriver:
    "Middle East risk lifted the oil-risk premium while FX policy coordination stayed in focus.",
  todaysSignal:
    "Markets are trading defensively as FX policy tension, Middle East risk, and upcoming U.S. inflation data dominate sentiment.",
  readThisFirst: [
    "U.S.-Japan FX coordination may affect yen intervention expectations.",
    "Middle East ceasefire risk is supporting oil prices.",
    "AI hardware demand remains strong, but valuation pressure is building.",
  ],
  marketSnapshot: {
    equityIndices: [
      {
        asset: "S&P 500",
        level: "7,580.06",
        dailyChange: "+0.42%",
        timestamp: "16:03 ET",
        driver:
          "Large-cap tech strength offset defensive positioning before inflation data.",
        source: "Yahoo Finance",
      },
      {
        asset: "Nikkei 225",
        level: "66,329.50",
        dailyChange: "-0.18%",
        timestamp: "15:45 JST",
        driver: "Yen volatility kept exporter sentiment mixed.",
        source: "Yahoo Finance",
      },
    ],
    fx: [
      {
        asset: "USD/JPY",
        level: "159.26",
        dailyChange: "+0.27%",
        timestamp: "08:45 JST",
        driver:
          "Yen weakened as U.S.-Japan yield spreads stayed wide and FX policy coordination remained in focus.",
        source: "Yahoo Finance",
      },
      {
        asset: "DXY",
        level: "98.94",
        dailyChange: "+0.11%",
        timestamp: "16:00 ET",
        driver: "Dollar demand held firm ahead of U.S. macro data.",
        source: "Yahoo Finance",
      },
    ],
    ratesBonds: [
      {
        asset: "U.S. 10Y Treasury yield",
        level: "4.453%",
        dailyChange: "+3 bps",
        timestamp: "14:59 ET",
        driver:
          "Rates stayed elevated as inflation risk remained the main macro constraint.",
        source: "Yahoo Finance",
      },
    ],
    commoditiesCryptoRisk: [
      {
        asset: "Brent crude",
        level: "91.12",
        dailyChange: "+0.68%",
        timestamp: "16:59 ET",
        driver:
          "Ceasefire uncertainty kept geopolitical risk premium in energy markets.",
        source: "Yahoo Finance",
      },
      {
        asset: "VIX",
        level: "15.32",
        dailyChange: "+0.09",
        timestamp: "16:15 ET",
        driver:
          "Options demand remained modest despite policy and geopolitical risk.",
        source: "Yahoo Finance",
      },
    ],
  },
  macroDataWatch: [
    "U.S. CPI remains the key near-term data release for rates and dollar direction.",
    "Japan inflation and wage signals remain important for BOJ expectations.",
  ],
  policySignalWatch: [
    "FX policy coordination between the U.S. and Japan remains relevant for USD/JPY.",
    "Defense spending and energy-security policy remain active themes in Europe.",
  ],
  topNewsByTopic: {
    marketsMacro: [
      {
        topic: "Markets & Macro",
        region: "U.S. / Japan",
        headline: "FX policy coordination stays central for yen expectations",
        summary:
          "Officials kept currency stability in focus as wide rate differentials continued to pressure the yen. Traders are watching whether verbal coordination turns into intervention risk.",
        whyItMatters:
          "USD/JPY remains a direct read-through for BOJ expectations, exporter sentiment, and dollar positioning.",
        sources: [
          { label: "Reuters", url: "https://www.reuters.com" },
          { label: "Bloomberg", url: "https://www.bloomberg.com" },
        ],
        priorityScore: 8.42,
        confidence: "High",
        mustRead: true,
      },
      {
        topic: "Markets & Macro",
        region: "Global",
        headline:
          "Inflation data keeps rates at the center of the market setup",
        summary:
          "Investors remain focused on whether upcoming price data confirms a slower disinflation path. Rate-sensitive equities and FX pairs are likely to stay reactive.",
        whyItMatters:
          "The inflation path drives the next move in yields, dollar strength, and equity multiples.",
        sources: [{ label: "Bloomberg", url: "https://www.bloomberg.com" }],
        priorityScore: 7.91,
        confidence: "Medium",
      },
    ],
    politicsPolicy: [
      {
        topic: "Politics & Policy",
        region: "Europe",
        headline:
          "Energy security and defense spending remain linked policy themes",
        summary:
          "European governments continued to frame energy resilience and defense investment as economic priorities. Fiscal pressure remains part of the policy tradeoff.",
        whyItMatters:
          "Defense and energy policy can affect industrial shares, fiscal risk, and regional growth assumptions.",
        sources: [
          { label: "FT", url: "https://www.ft.com" },
          { label: "Reuters", url: "https://www.reuters.com" },
        ],
        priorityScore: 7.64,
        confidence: "Medium",
      },
    ],
    warGeopoliticalRisk: [
      {
        topic: "War & Geopolitical Risk",
        region: "Middle East",
        headline: "Ceasefire uncertainty keeps oil-risk premium elevated",
        summary:
          "Headlines around regional escalation and ceasefire talks remained mixed. Energy markets continued to price some disruption risk.",
        whyItMatters:
          "Oil risk premium feeds directly into inflation expectations and risk appetite.",
        sources: [
          { label: "Reuters", url: "https://www.reuters.com" },
          { label: "Al Jazeera", url: "https://www.aljazeera.com" },
        ],
        priorityScore: 8.11,
        confidence: "Medium",
        mustRead: true,
      },
    ],
    technologyAI: [
      {
        topic: "Technology & AI",
        region: "U.S.",
        headline:
          "AI hardware demand remains strong despite valuation pressure",
        summary:
          "AI infrastructure demand continues to support semiconductor and cloud-exposed names. The main investor concern is whether earnings growth can keep pace with valuations.",
        whyItMatters:
          "AI capex remains one of the strongest equity-market narratives, but valuation sensitivity is rising.",
        sources: [
          { label: "Bloomberg", url: "https://www.bloomberg.com" },
          { label: "FT", url: "https://www.ft.com" },
        ],
        priorityScore: 8.03,
        confidence: "High",
        mustRead: true,
      },
    ],
  },
  regionalRadar: [
    {
      region: "U.S.",
      sentence:
        "Fed speakers kept inflation risk in focus ahead of the next CPI release.",
      sources: [{ label: "Reuters", url: "https://www.reuters.com" }],
    },
    {
      region: "Japan",
      sentence:
        "FX policy coordination with the U.S. remains important for USD/JPY and BOJ expectations.",
      sources: [{ label: "Bloomberg", url: "https://www.bloomberg.com" }],
    },
    {
      region: "China / Hong Kong / Taiwan",
      sentence:
        "Mainland equity sentiment weakened as property and trade concerns continued.",
      sources: [{ label: "Reuters", url: "https://www.reuters.com" }],
    },
    {
      region: "Europe",
      sentence:
        "Energy prices and defense spending remained the main political-economic themes.",
      sources: [{ label: "FT", url: "https://www.ft.com" }],
    },
    {
      region: "Middle East",
      sentence: "Ceasefire uncertainty kept oil-risk premium elevated.",
      sources: [{ label: "Al Jazeera", url: "https://www.aljazeera.com" }],
    },
  ],
  toneFramingDifferences: [
    "U.S. coverage emphasized inflation and rates, while Asian coverage emphasized FX policy and trade risk.",
  ],
  techTendency: [
    "AI infrastructure remains constructive, but valuation language is becoming more cautious.",
  ],
  polymarketWatch: [],
  watchNext: [
    "U.S. CPI release",
    "BOJ official remarks",
    "Brent crude reaction to Middle East headlines",
  ],
  whyThisMattersToday:
    "The briefing points to a market still led by policy-sensitive macro variables: rates, FX, oil, and AI valuation risk. The highest-impact setup is any headline that shifts inflation expectations or central-bank reaction functions.",
};

export default function GPNewsBriefingEmail(props: Partial<BriefingEmail>) {
  const briefing = { ...sampleBriefing, ...props };
  const criticalityLabel = labelForCriticality(briefing.criticalityScore);
  const allNews = [
    ...briefing.topNewsByTopic.marketsMacro,
    ...briefing.topNewsByTopic.politicsPolicy,
    ...briefing.topNewsByTopic.warGeopoliticalRisk,
    ...briefing.topNewsByTopic.technologyAI,
  ];

  return (
    <Html>
      <Head />
      <Preview>
        GP Criticality {briefing.criticalityScore.toFixed(2)} / 10 -{" "}
        {briefing.mainDriver}
      </Preview>
      <Body style={styles.body}>
        <Container style={styles.container}>
          <Section style={styles.header}>
            <Text style={styles.kicker}>GP News Intelligence Desk</Text>
            <Heading style={styles.title}>Daily Market Briefing</Heading>
            <Text style={styles.scoreLine}>
              GP Criticality Score:{" "}
              <span style={styles.strong}>
                {briefing.criticalityScore.toFixed(2)} / 10
              </span>{" "}
              - {criticalityLabel}
            </Text>
            <Row>
              <Column style={styles.metaColumn}>
                <Text style={styles.metaLabel}>Priority Level</Text>
                <Text style={styles.metaValue}>{briefing.priorityLevel}</Text>
              </Column>
              <Column style={styles.metaColumn}>
                <Text style={styles.metaLabel}>High-Priority Tag</Text>
                <Text style={styles.metaValue}>
                  {briefing.highPriorityTag ? "Applied" : "Not Applied"}
                </Text>
              </Column>
            </Row>
            <Text style={styles.driver}>
              <span style={styles.strong}>Main Driver:</span>{" "}
              {briefing.mainDriver}
            </Text>
          </Section>

          <Section style={styles.section}>
            <SectionTitle>Today's Signal</SectionTitle>
            <Text style={styles.lead}>{briefing.todaysSignal}</Text>
          </Section>

          <Section style={styles.panel}>
            <SectionTitle>Read This First</SectionTitle>
            {briefing.readThisFirst.slice(0, 3).map((item, index) => (
              <Text key={item} style={styles.numberedItem}>
                <span style={styles.itemNumber}>{index + 1}.</span> {item}
              </Text>
            ))}
          </Section>

          <Section style={styles.index}>
            <Text style={styles.indexText}>
              Market Snapshot | Macro Data Watch | Policy Signal Watch | Top
              News | Regional Radar | Watch Next
            </Text>
          </Section>

          <Section style={styles.section}>
            <SectionTitle>Market Snapshot</SectionTitle>
            <MarketGroup
              title="A. Equity Indices"
              items={briefing.marketSnapshot.equityIndices}
            />
            <MarketGroup title="B. FX" items={briefing.marketSnapshot.fx} />
            <MarketGroup
              title="C. Rates / Bonds"
              items={briefing.marketSnapshot.ratesBonds}
            />
            <MarketGroup
              title="D. Commodities / Crypto / Risk"
              items={briefing.marketSnapshot.commoditiesCryptoRisk}
            />
          </Section>

          <TwoColumnList
            leftTitle="Macro Data Watch"
            leftItems={briefing.macroDataWatch}
            rightTitle="Policy Signal Watch"
            rightItems={briefing.policySignalWatch}
          />

          <Section style={styles.section}>
            <SectionTitle>Top News By Topic</SectionTitle>
            <NewsGroup
              title="A. Markets & Macro"
              cards={briefing.topNewsByTopic.marketsMacro}
            />
            <NewsGroup
              title="B. Politics & Policy"
              cards={briefing.topNewsByTopic.politicsPolicy}
            />
            <NewsGroup
              title="C. War & Geopolitical Risk"
              cards={briefing.topNewsByTopic.warGeopoliticalRisk}
            />
            <NewsGroup
              title="D. Technology & AI"
              cards={briefing.topNewsByTopic.technologyAI}
            />
            <Text style={styles.countHint}>
              {allNews.length} full news cards shown.
            </Text>
          </Section>

          <Section style={styles.section}>
            <SectionTitle>Regional Radar</SectionTitle>
            {briefing.regionalRadar.slice(0, 8).map((item) => (
              <Text key={item.region} style={styles.radarItem}>
                <span style={styles.strong}>{item.region}</span> -{" "}
                {item.sentence}
                <br />
                <span style={styles.fieldLabel}>Sources:</span>{" "}
                <SourceLinks sources={item.sources} />
              </Text>
            ))}
          </Section>

          <OptionalList
            title="Tone / Framing Differences"
            items={briefing.toneFramingDifferences}
          />
          <OptionalList title="Tech Tendency" items={briefing.techTendency} />
          <OptionalList
            title="Polymarket Watch"
            items={briefing.polymarketWatch}
          />

          <Section style={styles.panel}>
            <SectionTitle>Watch Next</SectionTitle>
            {briefing.watchNext.slice(0, 3).map((item) => (
              <Text key={item} style={styles.bulletItem}>
                - {item}
              </Text>
            ))}
          </Section>

          <Section style={styles.section}>
            <SectionTitle>Why This Matters Today</SectionTitle>
            <Text style={styles.paragraph}>{briefing.whyThisMattersToday}</Text>
          </Section>

          <Hr style={styles.hr} />
          <Text style={styles.footer}>
            Generated for GP News. Market levels and timestamps should be
            checked against source data before publication.
          </Text>
        </Container>
      </Body>
    </Html>
  );
}

function SectionTitle({ children }: { children: React.ReactNode }) {
  return <Heading style={styles.sectionTitle}>{children}</Heading>;
}

function MarketGroup({ title, items }: { title: string; items: MarketItem[] }) {
  if (items.length === 0) {
    return null;
  }

  return (
    <Section style={styles.marketGroup}>
      <Heading style={styles.groupTitle}>{title}</Heading>
      {items.map((item) => (
        <Section key={`${title}-${item.asset}`} style={styles.marketCard}>
          <Text style={styles.marketLine}>
            <span style={styles.strong}>{item.asset}:</span> {item.level}
            {item.dailyChange ? ` (${item.dailyChange})` : ""}, {item.timestamp}
          </Text>
          <Text style={styles.detailLine}>
            <span style={styles.detailLabel}>Driver:</span> {item.driver}
          </Text>
          <Text style={styles.detailLine}>
            <span style={styles.detailLabel}>Source:</span> {item.source}
          </Text>
        </Section>
      ))}
    </Section>
  );
}

function TwoColumnList({
  leftTitle,
  leftItems,
  rightTitle,
  rightItems,
}: {
  leftTitle: string;
  leftItems: string[];
  rightTitle: string;
  rightItems: string[];
}) {
  return (
    <Section style={styles.section}>
      <Row>
        <Column style={styles.listColumn}>
          <SectionTitle>{leftTitle}</SectionTitle>
          {leftItems.map((item) => (
            <Text key={item} style={styles.bulletItem}>
              - {item}
            </Text>
          ))}
        </Column>
        <Column style={styles.listColumn}>
          <SectionTitle>{rightTitle}</SectionTitle>
          {rightItems.map((item) => (
            <Text key={item} style={styles.bulletItem}>
              - {item}
            </Text>
          ))}
        </Column>
      </Row>
    </Section>
  );
}

function NewsGroup({ title, cards }: { title: string; cards: NewsCard[] }) {
  if (cards.length === 0) {
    return null;
  }

  return (
    <Section style={styles.newsGroup}>
      <Heading style={styles.groupTitle}>{title}</Heading>
      {cards.map((card) => (
        <Section
          key={`${card.topic}-${card.region}-${card.headline}`}
          style={styles.newsCard}
        >
          <Text style={styles.badgeLine}>
            <Badge>{card.topic}</Badge>
            <Badge>{card.region}</Badge>
            {card.mustRead ? <Badge tone="dark">Must Read</Badge> : null}
          </Text>
          <Heading style={styles.newsHeadline}>{card.headline}</Heading>
          <Field label="Summary" value={card.summary} />
          <Field label="Why it matters" value={card.whyItMatters} />
          <Text style={styles.field}>
            <span style={styles.fieldLabel}>Sources:</span>{" "}
            <SourceLinks sources={card.sources} />
          </Text>
          <Row>
            <Column>
              <Text style={styles.field}>
                <span style={styles.fieldLabel}>Priority Score:</span>{" "}
                {card.priorityScore.toFixed(2)} / 10
              </Text>
            </Column>
            <Column>
              <Text style={styles.field}>
                <span style={styles.fieldLabel}>Confidence:</span>{" "}
                {card.confidence}
              </Text>
            </Column>
          </Row>
        </Section>
      ))}
    </Section>
  );
}

function SourceLinks({ sources }: { sources: SourceItem[] }) {
  return (
    <>
      {sources.map((source, index) => (
        <React.Fragment key={`${source.label}-${source.url ?? index}`}>
          {source.url ? (
            <Link href={source.url} style={styles.sourceLink}>
              {source.label}
            </Link>
          ) : (
            <span>{source.label}</span>
          )}
          {index < sources.length - 1 ? " | " : ""}
        </React.Fragment>
      ))}
    </>
  );
}

function Field({ label, value }: { label: string; value: string }) {
  return (
    <Text style={styles.field}>
      <span style={styles.fieldLabel}>{label}:</span> {value}
    </Text>
  );
}

function OptionalList({
  title,
  items,
}: {
  title: string;
  items?: string[] | undefined;
}) {
  if (!items || items.length === 0) {
    return null;
  }

  return (
    <Section style={styles.section}>
      <SectionTitle>{title}</SectionTitle>
      {items.map((item) => (
        <Text key={item} style={styles.bulletItem}>
          - {item}
        </Text>
      ))}
    </Section>
  );
}

function Badge({
  children,
  tone = "light",
}: {
  children: React.ReactNode;
  tone?: "light" | "dark";
}) {
  return (
    <span style={tone === "dark" ? styles.badgeDark : styles.badge}>
      {children}
    </span>
  );
}

function labelForCriticality(score: number): PriorityLevel {
  if (score >= 8) {
    return "Critical";
  }
  if (score >= 6) {
    return "Important";
  }
  if (score >= 3) {
    return "Watch";
  }
  return "Low";
}

const styles = {
  body: {
    margin: "0",
    backgroundColor: "#f4f6f8",
    color: "#1b1f23",
    fontFamily:
      '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif',
  },
  container: {
    width: "100%",
    maxWidth: "680px",
    margin: "0 auto",
    padding: "24px 14px 32px",
  },
  header: {
    backgroundColor: "#102033",
    borderRadius: "8px",
    padding: "24px",
    color: "#ffffff",
  },
  kicker: {
    margin: "0 0 8px",
    color: "#9fb6ce",
    fontSize: "13px",
    fontWeight: "700",
  },
  title: {
    margin: "0 0 16px",
    color: "#ffffff",
    fontSize: "28px",
    lineHeight: "34px",
    fontWeight: "800",
  },
  scoreLine: {
    margin: "0 0 14px",
    color: "#ffffff",
    fontSize: "16px",
    lineHeight: "24px",
  },
  metaColumn: {
    width: "50%",
    paddingRight: "12px",
  },
  metaLabel: {
    margin: "0 0 4px",
    color: "#9fb6ce",
    fontSize: "12px",
    lineHeight: "16px",
  },
  metaValue: {
    margin: "0 0 12px",
    color: "#ffffff",
    fontSize: "15px",
    lineHeight: "20px",
    fontWeight: "700",
  },
  driver: {
    margin: "4px 0 0",
    color: "#edf4fb",
    fontSize: "14px",
    lineHeight: "21px",
  },
  section: {
    padding: "20px 0 0",
  },
  panel: {
    marginTop: "18px",
    padding: "18px",
    backgroundColor: "#ffffff",
    border: "1px solid #d9e0e7",
    borderRadius: "8px",
  },
  sectionTitle: {
    margin: "0 0 10px",
    color: "#102033",
    fontSize: "18px",
    lineHeight: "24px",
    fontWeight: "800",
  },
  lead: {
    margin: "0",
    color: "#283542",
    fontSize: "16px",
    lineHeight: "24px",
  },
  numberedItem: {
    margin: "8px 0 0",
    color: "#283542",
    fontSize: "14px",
    lineHeight: "21px",
  },
  itemNumber: {
    color: "#102033",
    fontWeight: "800",
  },
  index: {
    marginTop: "16px",
    padding: "10px 12px",
    backgroundColor: "#e8eef4",
    borderRadius: "8px",
  },
  indexText: {
    margin: "0",
    color: "#425466",
    fontSize: "12px",
    lineHeight: "18px",
    fontWeight: "700",
  },
  marketGroup: {
    marginTop: "10px",
  },
  groupTitle: {
    margin: "14px 0 8px",
    color: "#425466",
    fontSize: "14px",
    lineHeight: "20px",
    fontWeight: "800",
  },
  marketCard: {
    marginTop: "8px",
    padding: "12px 14px",
    backgroundColor: "#ffffff",
    border: "1px solid #d9e0e7",
    borderRadius: "8px",
  },
  marketLine: {
    margin: "0 0 6px",
    color: "#1b1f23",
    fontSize: "14px",
    lineHeight: "20px",
  },
  detailLine: {
    margin: "3px 0 0",
    color: "#4d5b68",
    fontSize: "13px",
    lineHeight: "19px",
  },
  detailLabel: {
    color: "#283542",
    fontWeight: "700",
  },
  listColumn: {
    width: "50%",
    paddingRight: "12px",
    verticalAlign: "top",
  },
  bulletItem: {
    margin: "6px 0",
    color: "#283542",
    fontSize: "14px",
    lineHeight: "21px",
  },
  newsGroup: {
    marginTop: "12px",
  },
  newsCard: {
    marginTop: "10px",
    padding: "14px",
    backgroundColor: "#ffffff",
    border: "1px solid #d9e0e7",
    borderRadius: "8px",
  },
  badgeLine: {
    margin: "0 0 8px",
    lineHeight: "22px",
  },
  badge: {
    display: "inline-block",
    marginRight: "6px",
    marginBottom: "4px",
    padding: "3px 8px",
    backgroundColor: "#edf2f7",
    border: "1px solid #d9e0e7",
    borderRadius: "999px",
    color: "#425466",
    fontSize: "11px",
    lineHeight: "14px",
    fontWeight: "700",
  },
  badgeDark: {
    display: "inline-block",
    marginRight: "6px",
    marginBottom: "4px",
    padding: "3px 8px",
    backgroundColor: "#102033",
    border: "1px solid #102033",
    borderRadius: "999px",
    color: "#ffffff",
    fontSize: "11px",
    lineHeight: "14px",
    fontWeight: "700",
  },
  newsHeadline: {
    margin: "0 0 10px",
    color: "#102033",
    fontSize: "16px",
    lineHeight: "22px",
    fontWeight: "800",
  },
  field: {
    margin: "6px 0",
    color: "#283542",
    fontSize: "13px",
    lineHeight: "20px",
  },
  fieldLabel: {
    color: "#102033",
    fontWeight: "800",
  },
  countHint: {
    margin: "10px 0 0",
    color: "#6a7785",
    fontSize: "12px",
    lineHeight: "18px",
  },
  radarItem: {
    margin: "8px 0",
    padding: "10px 12px",
    backgroundColor: "#ffffff",
    border: "1px solid #d9e0e7",
    borderRadius: "8px",
    color: "#283542",
    fontSize: "14px",
    lineHeight: "20px",
  },
  paragraph: {
    margin: "0",
    color: "#283542",
    fontSize: "14px",
    lineHeight: "22px",
  },
  sourceLink: {
    color: "#0b66c3",
    textDecoration: "none",
    fontWeight: "700",
  },
  strong: {
    fontWeight: "800",
  },
  hr: {
    margin: "24px 0 12px",
    borderColor: "#d9e0e7",
  },
  footer: {
    margin: "0",
    color: "#6a7785",
    fontSize: "12px",
    lineHeight: "18px",
  },
} as const;
