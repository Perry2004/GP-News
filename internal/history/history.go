package history

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/Perry2004/GP-News/briefing"
)

const (
	SelectedNewsHistoryID = "selected_news#v1"
)

type DynamoDBAPI interface {
	Query(ctx context.Context, params *dynamodb.QueryInput, optFns ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error)
	PutItem(ctx context.Context, params *dynamodb.PutItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error)
}

type Store struct {
	tableName string
	client    DynamoDBAPI
}

type SelectedNewsRecord struct {
	BriefingHistoryID    string   `dynamodbav:"briefing_history_id"`
	BriefingEntryID      string   `dynamodbav:"briefing_entry_id"`
	EntityType           string   `dynamodbav:"entity_type"`
	BriefingDate         string   `dynamodbav:"briefing_date"`
	Session              string   `dynamodbav:"session"`
	RunID                string   `dynamodbav:"run_id"`
	BriefingAt           string   `dynamodbav:"briefing_at"`
	ExpiresAt            int64    `dynamodbav:"expires_at"`
	ArticleID            string   `dynamodbav:"article_id"`
	BucketID             string   `dynamodbav:"bucket_id"`
	BucketName           string   `dynamodbav:"bucket_name"`
	SourceURL            string   `dynamodbav:"source_url"`
	SourceName           string   `dynamodbav:"source_name"`
	SourceTitle          string   `dynamodbav:"source_title"`
	ExtractedTitle       string   `dynamodbav:"extracted_title"`
	ProcessedHeadline    string   `dynamodbav:"processed_headline"`
	Summary              string   `dynamodbav:"summary"`
	Entities             []string `dynamodbav:"entities"`
	Region               string   `dynamodbav:"region"`
	AssetClasses         []string `dynamodbav:"asset_classes"`
	MarketRelevanceScore float64  `dynamodbav:"market_relevance_score"`
	NoveltyScore         float64  `dynamodbav:"novelty_score"`
	PriorityScore        float64  `dynamodbav:"priority_score"`
	Confidence           string   `dynamodbav:"confidence"`
	WhyItMatters         string   `dynamodbav:"why_it_matters"`
	PossibleMarketImpact string   `dynamodbav:"possible_market_impact"`
	ReviewNote           string   `dynamodbav:"review_note"`
}

func NewDynamoDBStore(ctx context.Context, tableName string) (*Store, error) {
	tableName = strings.TrimSpace(tableName)
	if tableName == "" {
		return nil, fmt.Errorf("history table name is required")
	}
	awsCfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("load AWS config for briefing history: %w", err)
	}
	return NewStore(tableName, dynamodb.NewFromConfig(awsCfg)), nil
}

func NewStore(tableName string, client DynamoDBAPI) *Store {
	return &Store{tableName: strings.TrimSpace(tableName), client: client}
}

func (s *Store) RecentSelectedNews(ctx context.Context, since time.Time) ([]SelectedNewsRecord, error) {
	if s == nil || s.client == nil {
		return nil, fmt.Errorf("history store is not configured")
	}
	if strings.TrimSpace(s.tableName) == "" {
		return nil, fmt.Errorf("history table name is required")
	}

	cutoffEntryID := fmt.Sprintf("DATE#%s", since.Format(time.DateOnly))
	input := &dynamodb.QueryInput{
		TableName:              aws.String(s.tableName),
		KeyConditionExpression: aws.String("#history_id = :history_id AND #entry_id >= :cutoff_entry_id"),
		ExpressionAttributeNames: map[string]string{
			"#history_id": "briefing_history_id",
			"#entry_id":   "briefing_entry_id",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":history_id":      &types.AttributeValueMemberS{Value: SelectedNewsHistoryID},
			":cutoff_entry_id": &types.AttributeValueMemberS{Value: cutoffEntryID},
		},
		ScanIndexForward: aws.Bool(true),
	}

	var records []SelectedNewsRecord
	for {
		output, err := s.client.Query(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("query briefing history: %w", err)
		}
		var page []SelectedNewsRecord
		if err := attributevalue.UnmarshalListOfMaps(output.Items, &page); err != nil {
			return nil, fmt.Errorf("decode briefing history records: %w", err)
		}
		records = append(records, page...)
		if len(output.LastEvaluatedKey) == 0 {
			break
		}
		input.ExclusiveStartKey = output.LastEvaluatedKey
	}
	return records, nil
}

func (s *Store) PutSelectedNews(ctx context.Context, records []SelectedNewsRecord) error {
	if len(records) == 0 {
		return nil
	}
	if s == nil || s.client == nil {
		return fmt.Errorf("history store is not configured")
	}
	if strings.TrimSpace(s.tableName) == "" {
		return fmt.Errorf("history table name is required")
	}

	for _, record := range records {
		item, err := attributevalue.MarshalMap(record)
		if err != nil {
			return fmt.Errorf("encode briefing history record %q: %w", record.BriefingEntryID, err)
		}
		_, err = s.client.PutItem(ctx, &dynamodb.PutItemInput{
			TableName: aws.String(s.tableName),
			Item:      item,
		})
		if err != nil {
			return fmt.Errorf("put briefing history record %q: %w", record.BriefingEntryID, err)
		}
	}
	return nil
}

func BuildSelectedNewsRecords(input briefing.BriefingInput, articles []briefing.ArticleInput, runID string, now time.Time, ttlDays int) []SelectedNewsRecord {
	if ttlDays <= 0 {
		ttlDays = 14
	}
	runID = strings.TrimSpace(runID)
	if runID == "" {
		runID = now.UTC().Format("20060102T150405Z")
	}

	articleByID := make(map[string]briefing.ArticleInput, len(articles))
	for _, article := range articles {
		articleByID[article.ID] = article
	}

	expiresAt := now.UTC().Add(time.Duration(ttlDays) * 24 * time.Hour).Unix()
	records := make([]SelectedNewsRecord, 0, len(input.ReviewedNews))
	for _, reviewed := range input.ReviewedNews {
		news := reviewed.News
		article := articleByID[news.ArticleID]
		articleKey := stableArticleKey(news.ArticleID, news.SourceURL, news.Headline)
		records = append(records, SelectedNewsRecord{
			BriefingHistoryID:    SelectedNewsHistoryID,
			BriefingEntryID:      fmt.Sprintf("DATE#%s#SESSION#%s#RUN#%s#ARTICLE#%s", input.BriefingDate, sanitizeKeyPart(input.Session), sanitizeKeyPart(runID), articleKey),
			EntityType:           "selected_news",
			BriefingDate:         input.BriefingDate,
			Session:              input.Session,
			RunID:                runID,
			BriefingAt:           now.UTC().Format(time.RFC3339),
			ExpiresAt:            expiresAt,
			ArticleID:            news.ArticleID,
			BucketID:             article.BucketID,
			BucketName:           article.BucketName,
			SourceURL:            news.SourceURL,
			SourceName:           news.SourceName,
			SourceTitle:          article.Title,
			ExtractedTitle:       article.ExtractedTitle,
			ProcessedHeadline:    news.Headline,
			Summary:              news.Summary,
			Entities:             news.Entities,
			Region:               news.Region,
			AssetClasses:         news.AssetClasses,
			MarketRelevanceScore: news.MarketRelevanceScore,
			NoveltyScore:         news.NoveltyScore,
			PriorityScore:        reviewed.PriorityScore,
			Confidence:           news.Confidence,
			WhyItMatters:         news.WhyItMatters,
			PossibleMarketImpact: news.PossibleMarketImpact,
			ReviewNote:           reviewed.ReviewNote,
		})
	}
	return records
}

func DedupeRecentNews(records []SelectedNewsRecord) []briefing.BriefingHistoryDedupeRecentNews {
	recent := make([]briefing.BriefingHistoryDedupeRecentNews, 0, len(records))
	for _, record := range records {
		recent = append(recent, briefing.BriefingHistoryDedupeRecentNews{
			HistoryEntryID:       record.BriefingEntryID,
			BriefingDate:         record.BriefingDate,
			Session:              record.Session,
			SourceURL:            record.SourceURL,
			SourceName:           record.SourceName,
			ProcessedHeadline:    record.ProcessedHeadline,
			Summary:              record.Summary,
			Entities:             record.Entities,
			Region:               record.Region,
			AssetClasses:         record.AssetClasses,
			WhyItMatters:         record.WhyItMatters,
			PossibleMarketImpact: record.PossibleMarketImpact,
			ReviewNote:           record.ReviewNote,
		})
	}
	return recent
}

func stableArticleKey(articleID string, values ...string) string {
	if id := sanitizeKeyPart(articleID); id != "" {
		return id
	}
	hash := sha256.Sum256([]byte(strings.Join(values, "\n")))
	return hex.EncodeToString(hash[:])[:16]
}

func sanitizeKeyPart(value string) string {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, "#", "_")
	value = strings.ReplaceAll(value, "/", "_")
	value = strings.ReplaceAll(value, " ", "_")
	return value
}
