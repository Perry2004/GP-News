package history

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/Perry2004/GP-News/briefing"
)

type fakeDynamoDB struct {
	queryInput *dynamodb.QueryInput
	queryOut   *dynamodb.QueryOutput
	queryErr   error
	putInputs  []*dynamodb.PutItemInput
	putErr     error
}

func (f *fakeDynamoDB) Query(ctx context.Context, input *dynamodb.QueryInput, optFns ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error) {
	f.queryInput = input
	if f.queryErr != nil {
		return nil, f.queryErr
	}
	if f.queryOut != nil {
		return f.queryOut, nil
	}
	return &dynamodb.QueryOutput{}, nil
}

func (f *fakeDynamoDB) PutItem(ctx context.Context, input *dynamodb.PutItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error) {
	f.putInputs = append(f.putInputs, input)
	if f.putErr != nil {
		return nil, f.putErr
	}
	return &dynamodb.PutItemOutput{}, nil
}

func TestRecentSelectedNewsQueriesByHistoryIDAndLookbackCutoff(t *testing.T) {
	item, err := attributevalue.MarshalMap(SelectedNewsRecord{
		BriefingHistoryID: SelectedNewsHistoryID,
		BriefingEntryID:   "DATE#2026-05-29#SESSION#Morning#RUN#r1#ARTICLE#a1",
		EntityType:        "selected_news",
		BriefingDate:      "2026-05-29",
		Session:           "Morning",
		Summary:           "Prior summary.",
	})
	if err != nil {
		t.Fatalf("MarshalMap() error = %v", err)
	}
	client := &fakeDynamoDB{queryOut: &dynamodb.QueryOutput{Items: []map[string]types.AttributeValue{item}}}
	store := NewStore("history-table", client)

	records, err := store.RecentSelectedNews(context.Background(), time.Date(2026, 5, 29, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("RecentSelectedNews() error = %v", err)
	}
	if len(records) != 1 || records[0].Summary != "Prior summary." {
		t.Fatalf("records = %#v, want decoded prior summary", records)
	}

	if got := aws.ToString(client.queryInput.TableName); got != "history-table" {
		t.Fatalf("TableName = %q, want history-table", got)
	}
	if got := client.queryInput.ExpressionAttributeNames["#history_id"]; got != "briefing_history_id" {
		t.Fatalf("history key name = %q, want briefing_history_id", got)
	}
	if got := client.queryInput.ExpressionAttributeNames["#entry_id"]; got != "briefing_entry_id" {
		t.Fatalf("entry key name = %q, want briefing_entry_id", got)
	}
	historyID := client.queryInput.ExpressionAttributeValues[":history_id"].(*types.AttributeValueMemberS).Value
	if historyID != SelectedNewsHistoryID {
		t.Fatalf("history id = %q, want %q", historyID, SelectedNewsHistoryID)
	}
	cutoff := client.queryInput.ExpressionAttributeValues[":cutoff_entry_id"].(*types.AttributeValueMemberS).Value
	if cutoff != "DATE#2026-05-29" {
		t.Fatalf("cutoff = %q, want DATE#2026-05-29", cutoff)
	}
}

func TestBuildSelectedNewsRecordsExcludesExtractedContentAndWritesTTL(t *testing.T) {
	now := time.Date(2026, 5, 30, 15, 4, 5, 0, time.UTC)
	records := BuildSelectedNewsRecords(briefing.BriefingInput{
		BriefingDate: "2026-05-30",
		Session:      "Morning",
		ReviewedNews: []briefing.ReviewedNews{
			{
				News: briefing.ProcessedNews{
					ArticleID:            "a1",
					Headline:             "Rates headline",
					Summary:              "Rates summary.",
					Entities:             []string{"Federal Reserve"},
					Region:               "U.S.",
					AssetClasses:         []string{"rates"},
					MarketRelevanceScore: 8.5,
					NoveltyScore:         7.5,
					WhyItMatters:         "Rates matter.",
					PossibleMarketImpact: "Yields may move.",
					Confidence:           "High",
					SourceURL:            "https://example.test/rates",
					SourceName:           "example.test",
				},
				PriorityScore: 9.1,
				ReviewNote:    "Selected after review.",
			},
		},
	}, []briefing.ArticleInput{
		{
			ID:               "a1",
			BucketID:         "markets_macro",
			BucketName:       "Markets & Macro",
			Title:            "Original rates title",
			ExtractedTitle:   "Extracted rates title",
			ExtractedContent: "secret full extracted content",
		},
	}, "run-1", now, 14)

	if len(records) != 1 {
		t.Fatalf("record count = %d, want 1", len(records))
	}
	record := records[0]
	if record.ExpiresAt != now.Add(14*24*time.Hour).Unix() {
		t.Fatalf("ExpiresAt = %d, want 14-day TTL", record.ExpiresAt)
	}
	if !strings.Contains(record.BriefingEntryID, "DATE#2026-05-30#SESSION#Morning#RUN#run-1#ARTICLE#a1") {
		t.Fatalf("BriefingEntryID = %q", record.BriefingEntryID)
	}

	item, err := attributevalue.MarshalMap(record)
	if err != nil {
		t.Fatalf("MarshalMap() error = %v", err)
	}
	if _, ok := item["extracted_content"]; ok {
		t.Fatal("record unexpectedly includes extracted_content")
	}
	if got := item["expires_at"].(*types.AttributeValueMemberN).Value; got == "" {
		t.Fatal("expires_at was not marshaled")
	}

	client := &fakeDynamoDB{}
	store := NewStore("history-table", client)
	if err := store.PutSelectedNews(context.Background(), records); err != nil {
		t.Fatalf("PutSelectedNews() error = %v", err)
	}
	if len(client.putInputs) != 1 {
		t.Fatalf("put count = %d, want 1", len(client.putInputs))
	}
	if _, ok := client.putInputs[0].Item["briefing_history_id"]; !ok {
		t.Fatal("put item missing briefing_history_id")
	}
	if _, ok := client.putInputs[0].Item["briefing_entry_id"]; !ok {
		t.Fatal("put item missing briefing_entry_id")
	}
	if _, ok := client.putInputs[0].Item["expires_at"]; !ok {
		t.Fatal("put item missing expires_at")
	}
}
