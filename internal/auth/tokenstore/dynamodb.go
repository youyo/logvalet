package tokenstore

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/youyo/logvalet/internal/auth"
)

// DynamoDBAPI は DynamoDB クライアントが満たすべき最小インターフェース。
// テスト時にモック注入するために定義する。
type DynamoDBAPI interface {
	GetItem(ctx context.Context, params *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error)
	PutItem(ctx context.Context, params *dynamodb.PutItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error)
	DeleteItem(ctx context.Context, params *dynamodb.DeleteItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DeleteItemOutput, error)
}

// errDynamoDBStoreClosed は Close 済みの DynamoDBStore への操作時に返されるエラー。
var errDynamoDBStoreClosed = errors.New("dynamodb token store: store is closed")

// DynamoDBStore は DynamoDB ベースの auth.TokenStore 実装。
//
// 用途:
//   - Lambda 本命
//   - VPC 不要
//   - マルチインスタンス耐性
//
// テーブル設計:
//   - PK: "USER#<userID>#<provider>#<tenant>"（単一 PK、SK なし）
type DynamoDBStore struct {
	client    DynamoDBAPI
	tableName string
	mu        sync.RWMutex
	closed    bool
}

// NewDynamoDBStore は新しい DynamoDBStore を返す。
// 本番用コンストラクタで、AWS SDK のデフォルト設定から DynamoDB クライアントを生成する。
func NewDynamoDBStore(tableName, region string) (*DynamoDBStore, error) {
	cfg, err := config.LoadDefaultConfig(context.Background(), config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("dynamodb token store: load config: %w", err)
	}

	client := dynamodb.NewFromConfig(cfg)
	return &DynamoDBStore{
		client:    client,
		tableName: tableName,
	}, nil
}

// NewDynamoDBStoreWithClient はテスト用コンストラクタで、DynamoDBAPI の任意実装を注入できる。
func NewDynamoDBStoreWithClient(tableName string, client DynamoDBAPI) *DynamoDBStore {
	return &DynamoDBStore{
		client:    client,
		tableName: tableName,
	}
}

// dynamoDBPK は DynamoDB の PK を生成する。
func dynamoDBPK(userID, provider, tenant string) string {
	return "USER#" + userID + "#" + provider + "#" + tenant
}

// Get は指定されたキーに対応する TokenRecord を返す。
// レコードが存在しない場合は nil, nil を返す（エラーではない）。
func (s *DynamoDBStore) Get(ctx context.Context, userID, provider, tenant string) (*auth.TokenRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return nil, errDynamoDBStoreClosed
	}

	pk := dynamoDBPK(userID, provider, tenant)
	out, err := s.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: &s.tableName,
		Key: map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: pk},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("dynamodb token store: get: %w", err)
	}

	if out.Item == nil || len(out.Item) == 0 {
		return nil, nil
	}

	rec, err := itemToTokenRecord(out.Item)
	if err != nil {
		return nil, fmt.Errorf("dynamodb token store: unmarshal: %w", err)
	}

	return rec, nil
}

// Put はレコードを保存する。同じキーが存在する場合は上書き（upsert）する。
func (s *DynamoDBStore) Put(ctx context.Context, record *auth.TokenRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return errDynamoDBStoreClosed
	}

	pk := dynamoDBPK(record.UserID, record.Provider, record.Tenant)
	item := tokenRecordToItem(pk, record)

	_, err := s.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: &s.tableName,
		Item:      item,
	})
	if err != nil {
		return fmt.Errorf("dynamodb token store: put: %w", err)
	}

	return nil
}

// Delete は指定されたキーのレコードを削除する。
// レコードが存在しない場合もエラーにならない。
func (s *DynamoDBStore) Delete(ctx context.Context, userID, provider, tenant string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return errDynamoDBStoreClosed
	}

	pk := dynamoDBPK(userID, provider, tenant)
	_, err := s.client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: &s.tableName,
		Key: map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: pk},
		},
	})
	if err != nil {
		return fmt.Errorf("dynamodb token store: delete: %w", err)
	}

	return nil
}

// Close はストアを閉じる。DynamoDB は接続管理不要のため no-op だが、
// closed フラグを立てて以降の操作をエラーにする。冪等。
func (s *DynamoDBStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.closed = true
	return nil
}

// tokenRecordToItem は TokenRecord を DynamoDB 属性マップに変換する。
func tokenRecordToItem(pk string, rec *auth.TokenRecord) map[string]types.AttributeValue {
	return map[string]types.AttributeValue{
		"pk":               &types.AttributeValueMemberS{Value: pk},
		"user_id":          &types.AttributeValueMemberS{Value: rec.UserID},
		"provider":         &types.AttributeValueMemberS{Value: rec.Provider},
		"tenant":           &types.AttributeValueMemberS{Value: rec.Tenant},
		"access_token":     &types.AttributeValueMemberS{Value: rec.AccessToken},
		"refresh_token":    &types.AttributeValueMemberS{Value: rec.RefreshToken},
		"token_type":       &types.AttributeValueMemberS{Value: rec.TokenType},
		"scope":            &types.AttributeValueMemberS{Value: rec.Scope},
		"expiry":           &types.AttributeValueMemberS{Value: rec.Expiry.UTC().Format(time.RFC3339)},
		"provider_user_id": &types.AttributeValueMemberS{Value: rec.ProviderUserID},
		"created_at":       &types.AttributeValueMemberS{Value: rec.CreatedAt.UTC().Format(time.RFC3339)},
		"updated_at":       &types.AttributeValueMemberS{Value: rec.UpdatedAt.UTC().Format(time.RFC3339)},
	}
}

// itemToTokenRecord は DynamoDB 属性マップを TokenRecord に変換する。
func itemToTokenRecord(item map[string]types.AttributeValue) (*auth.TokenRecord, error) {
	rec := &auth.TokenRecord{
		UserID:         getStringAttr(item, "user_id"),
		Provider:       getStringAttr(item, "provider"),
		Tenant:         getStringAttr(item, "tenant"),
		AccessToken:    getStringAttr(item, "access_token"),
		RefreshToken:   getStringAttr(item, "refresh_token"),
		TokenType:      getStringAttr(item, "token_type"),
		Scope:          getStringAttr(item, "scope"),
		ProviderUserID: getStringAttr(item, "provider_user_id"),
	}

	var err error
	rec.Expiry, err = time.Parse(time.RFC3339, getStringAttr(item, "expiry"))
	if err != nil {
		return nil, fmt.Errorf("parse expiry: %w", err)
	}
	rec.CreatedAt, err = time.Parse(time.RFC3339, getStringAttr(item, "created_at"))
	if err != nil {
		return nil, fmt.Errorf("parse created_at: %w", err)
	}
	rec.UpdatedAt, err = time.Parse(time.RFC3339, getStringAttr(item, "updated_at"))
	if err != nil {
		return nil, fmt.Errorf("parse updated_at: %w", err)
	}

	return rec, nil
}

// getStringAttr は DynamoDB 属性マップから文字列値を安全に取得する。
func getStringAttr(item map[string]types.AttributeValue, key string) string {
	v, ok := item[key]
	if !ok {
		return ""
	}
	sv, ok := v.(*types.AttributeValueMemberS)
	if !ok {
		return ""
	}
	return sv.Value
}
