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
  daily_change?: string;
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
  why_it_matters: string;
  sources: SourceItem[];
  priority_score: number;
  confidence: Confidence;
  must_read?: boolean;
};

type RegionalRadarItem = {
  region: string;
  sentence: string;
  sources: SourceItem[];
};

export type BriefingEmail = {
  subject: string;
  criticality_score: number;
  priority_level: PriorityLevel;
  high_priority_tag: boolean;
  main_driver: string;
  todays_signal: string;
  read_this_first: string[];
  market_snapshot: {
    equity_indices: MarketItem[];
    fx: MarketItem[];
    rates_bonds: MarketItem[];
    commodities_crypto_risk: MarketItem[];
  };
  macro_data_watch: string[];
  policy_signal_watch: string[];
  top_news_by_topic: {
    markets_macro: NewsCard[];
    politics_policy: NewsCard[];
    war_geopolitical_risk: NewsCard[];
    technology_ai: NewsCard[];
  };
  regional_radar: RegionalRadarItem[];
  tone_framing_differences?: string[];
  tech_tendency?: string[];
  polymarket_watch?: string[];
  watch_next: string[];
  why_this_matters_today: string;
};

const sampleBriefing: BriefingEmail = {
  subject: "GP News Briefing - Morning - GP Criticality 8.42",
  criticality_score: 8.42,
  priority_level: "Critical",
  high_priority_tag: true,
  main_driver:
    "Middle East risk lifted the oil-risk premium while FX policy coordination stayed in focus.",
  todays_signal:
    "Markets are trading defensively as FX policy tension, Middle East risk, and upcoming U.S. inflation data dominate sentiment.",
  read_this_first: [
    "U.S.-Japan FX coordination may affect yen intervention expectations.",
    "Middle East ceasefire risk is supporting oil prices.",
    "AI hardware demand remains strong, but valuation pressure is building.",
  ],
  market_snapshot: {
    equity_indices: [
      {
        asset: "S&P 500",
        level: "7,580.06",
        daily_change: "+31.84 (+0.42%)",
        timestamp: "16:03 ET",
        driver:
          "Large-cap tech strength offset defensive positioning before inflation data.",
        source: "Yahoo Finance",
      },
    ],
    fx: [
      {
        asset: "USD/JPY",
        level: "159.26",
        daily_change: "+0.43 (+0.27%)",
        timestamp: "08:45 JST",
        driver:
          "Yen weakened as U.S.-Japan yield spreads stayed wide and FX policy coordination remained in focus.",
        source: "Yahoo Finance",
      },
    ],
    rates_bonds: [
      {
        asset: "U.S. 10Y Treasury yield",
        level: "4.453%",
        daily_change: "+0.03 (+0.68%)",
        timestamp: "14:59 ET",
        driver:
          "Rates stayed elevated as inflation risk remained the main macro constraint.",
        source: "Yahoo Finance",
      },
    ],
    commodities_crypto_risk: [
      {
        asset: "Brent crude",
        level: "91.12",
        daily_change: "+0.61 (+0.68%)",
        timestamp: "16:59 ET",
        driver:
          "Ceasefire uncertainty kept geopolitical risk premium in energy markets.",
        source: "Yahoo Finance",
      },
    ],
  },
  macro_data_watch: [
    "U.S. CPI remains the key near-term data release for rates and dollar direction.",
  ],
  policy_signal_watch: [
    "FX policy coordination remains a key watch item for USD/JPY.",
  ],
  top_news_by_topic: {
    markets_macro: [
      {
        topic: "Markets & Macro",
        region: "Japan / U.S.",
        headline: "FX coordination language keeps yen intervention risk alive",
        summary:
          "Officials continued to frame currency stability as an important policy objective.",
        why_it_matters:
          "USD/JPY is still one of the cleanest expressions of the global rates and policy divergence trade.",
        sources: [{ label: "Reuters", url: "https://www.reuters.com" }],
        priority_score: 8.42,
        confidence: "High",
        must_read: true,
      },
    ],
    politics_policy: [],
    war_geopolitical_risk: [],
    technology_ai: [],
  },
  regional_radar: [
    {
      region: "U.S.",
      sentence:
        "Fed speakers kept inflation risk in focus ahead of the next CPI release.",
      sources: [{ label: "Reuters", url: "https://www.reuters.com" }],
    },
  ],
  tone_framing_differences: [
    "U.S. coverage emphasized inflation and rates, while Asian coverage emphasized FX policy and trade risk.",
  ],
  tech_tendency: [
    "AI infrastructure remains constructive, but valuation language is becoming more cautious.",
  ],
  polymarket_watch: [],
  watch_next: ["U.S. CPI release", "BOJ official remarks"],
  why_this_matters_today:
    "The briefing points to a market still led by policy-sensitive macro variables: rates, FX, oil, and AI valuation risk.",
};

function tpl(action: string) {
  return `{{${action}}}`;
}

function T({ action }: { action: string }) {
  return <>{tpl(action)}</>;
}

export default function GPNewsBriefingEmail(_props: Partial<BriefingEmail>) {
  void sampleBriefing;

  return (
    <Html>
      <Head>
        <title>{tpl(".subject")}</title>
      </Head>
      <Preview>{tpl(".subject")}</Preview>
      <Body style={styles.body}>
        <Container style={styles.container}>
          <Section style={styles.header}>
            <Text style={styles.kicker}>GP News Intelligence Desk</Text>
            <Heading style={styles.title}>Daily Market Briefing</Heading>
            <Text style={styles.scoreLine}>
              GP Criticality Score:{" "}
              <span style={styles.strong}>{tpl(".criticality_score")} / 10</span>{" "}
              - {tpl(".priority_level")}
            </Text>
            <Row>
              <Column style={styles.metaColumn}>
                <Text style={styles.metaLabel}>Priority Level</Text>
                <Text style={styles.metaValue}>{tpl(".priority_level")}</Text>
              </Column>
              <Column style={styles.metaColumn}>
                <Text style={styles.metaLabel}>High-Priority Tag</Text>
                <Text style={styles.metaValue}>
                  <T action="if .high_priority_tag" />
                  Applied
                  <T action="else" />
                  Not Applied
                  <T action="end" />
                </Text>
              </Column>
            </Row>
            <Text style={styles.driver}>
              <span style={styles.strong}>Main Driver:</span>{" "}
              {tpl(".main_driver")}
            </Text>
          </Section>

          <Section style={styles.section}>
            <SectionTitle>Today's Signal</SectionTitle>
            <Text style={styles.lead}>{tpl(".todays_signal")}</Text>
          </Section>

          <Section style={styles.panel}>
            <SectionTitle>Read This First</SectionTitle>
            <T action="range $index, $item := .read_this_first" />
            <Text style={styles.numberedItem}>
              <span style={styles.itemNumber}>{tpl("inc $index")}.</span>{" "}
              {tpl("$item")}
            </Text>
            <T action="end" />
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
              range=".market_snapshot.equity_indices"
            />
            <MarketGroup title="B. FX" range=".market_snapshot.fx" />
            <MarketGroup
              title="C. Rates / Bonds"
              range=".market_snapshot.rates_bonds"
            />
            <MarketGroup
              title="D. Commodities / Crypto / Risk"
              range=".market_snapshot.commodities_crypto_risk"
            />
          </Section>

          <TwoColumnList
            leftTitle="Macro Data Watch"
            leftRange=".macro_data_watch"
            rightTitle="Policy Signal Watch"
            rightRange=".policy_signal_watch"
          />

          <Section style={styles.section}>
            <SectionTitle>Top News By Topic</SectionTitle>
            <NewsGroup title="A. Markets & Macro" range=".top_news_by_topic.markets_macro" />
            <NewsGroup title="B. Politics & Policy" range=".top_news_by_topic.politics_policy" />
            <NewsGroup
              title="C. War & Geopolitical Risk"
              range=".top_news_by_topic.war_geopolitical_risk"
            />
            <NewsGroup title="D. Technology & AI" range=".top_news_by_topic.technology_ai" />
            <Text style={styles.countHint}>
              {tpl(".full_news_card_count")} full news cards shown.
            </Text>
          </Section>

          <Section style={styles.section}>
            <SectionTitle>Regional Radar</SectionTitle>
            <T action="range .regional_radar" />
            <Text style={styles.radarItem}>
              <span style={styles.strong}>{tpl(".region")}</span> -{" "}
              {tpl(".sentence")}
              <br />
              <span style={styles.fieldLabel}>Sources:</span>{" "}
              <SourceLinks />
            </Text>
            <T action="end" />
          </Section>

          <OptionalList title="Tone / Framing Differences" range=".tone_framing_differences" />
          <OptionalList title="Tech Tendency" range=".tech_tendency" />
          <OptionalList title="Polymarket Watch" range=".polymarket_watch" />

          <Section style={styles.panel}>
            <SectionTitle>Watch Next</SectionTitle>
            <T action="range .watch_next" />
            <Text style={styles.bulletItem}>- {tpl(".")}</Text>
            <T action="end" />
          </Section>

          <Section style={styles.section}>
            <SectionTitle>Why This Matters Today</SectionTitle>
            <Text style={styles.paragraph}>{tpl(".why_this_matters_today")}</Text>
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

function MarketGroup({ title, range }: { title: string; range: string }) {
  return (
    <Section style={styles.marketGroup}>
      <Heading style={styles.groupTitle}>{title}</Heading>
      <T action={`range ${range}`} />
      <Section style={styles.marketCard}>
        <Text style={styles.marketLine}>
          <span style={styles.strong}>{tpl(".asset")}:</span> {tpl(".level")}
          <T action="if .daily_change" />{" "}
          <span style={styles.marketChange}>{tpl(".daily_change")}</span>
          <T action="end" />, {tpl(".timestamp")}
        </Text>
        <Text style={styles.detailLine}>
          <span style={styles.detailLabel}>Driver:</span> {tpl(".driver")}
        </Text>
        <Text style={styles.detailLine}>
          <span style={styles.detailLabel}>Source:</span> {tpl(".source")}
        </Text>
      </Section>
      <T action="end" />
    </Section>
  );
}

function TwoColumnList({
  leftTitle,
  leftRange,
  rightTitle,
  rightRange,
}: {
  leftTitle: string;
  leftRange: string;
  rightTitle: string;
  rightRange: string;
}) {
  return (
    <Section style={styles.section}>
      <Row>
        <Column style={styles.listColumn}>
          <SectionTitle>{leftTitle}</SectionTitle>
          <T action={`range ${leftRange}`} />
          <Text style={styles.bulletItem}>- {tpl(".")}</Text>
          <T action="end" />
        </Column>
        <Column style={styles.listColumn}>
          <SectionTitle>{rightTitle}</SectionTitle>
          <T action={`range ${rightRange}`} />
          <Text style={styles.bulletItem}>- {tpl(".")}</Text>
          <T action="end" />
        </Column>
      </Row>
    </Section>
  );
}

function NewsGroup({ title, range }: { title: string; range: string }) {
  return (
    <Section style={styles.newsGroup}>
      <Heading style={styles.groupTitle}>{title}</Heading>
      <T action={`range ${range}`} />
      <Section style={styles.newsCard}>
        <Text style={styles.badgeLine}>
          <Badge>{tpl(".topic")}</Badge>
          <Badge>{tpl(".region")}</Badge>
          <T action="if .must_read" />
          <Badge tone="dark">Must Read</Badge>
          <T action="end" />
        </Text>
        <Heading style={styles.newsHeadline}>{tpl(".headline")}</Heading>
        <Field label="Summary" value={tpl(".summary")} />
        <Field label="Why it matters" value={tpl(".why_it_matters")} />
        <Text style={styles.field}>
          <span style={styles.fieldLabel}>Sources:</span> <SourceLinks />
        </Text>
        <Row>
          <Column>
            <Text style={styles.field}>
              <span style={styles.fieldLabel}>Priority Score:</span>{" "}
              {tpl(".priority_score")} / 10
            </Text>
          </Column>
          <Column>
            <Text style={styles.field}>
              <span style={styles.fieldLabel}>Confidence:</span>{" "}
              {tpl(".confidence")}
            </Text>
          </Column>
        </Row>
      </Section>
      <T action="end" />
    </Section>
  );
}

function SourceLinks() {
  return (
    <>
      <T action="range .sources" />
      <T action="if .url" />
      <Link href={tpl(".url")} style={styles.sourceLink}>
        {tpl(".label")}
      </Link>
      <T action="else" />
      <span>{tpl(".label")}</span>
      <T action="end" />{" "}
      <T action="end" />
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

function OptionalList({ title, range }: { title: string; range: string }) {
  return (
    <>
      <T action={`if ${range}`} />
      <Section style={styles.section}>
        <SectionTitle>{title}</SectionTitle>
        <T action={`range ${range}`} />
        <Text style={styles.bulletItem}>- {tpl(".")}</Text>
        <T action="end" />
      </Section>
      <T action="end" />
    </>
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
  marketChange: {
    color: "#425466",
    fontWeight: "700",
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
