package space

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// DynamoDBSpaceAPI は DynamoDB クライアントが満たすべき最小インターフェース。
type DynamoDBSpaceAPI interface {
	GetItem(ctx context.Context, params *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error)
	PutItem(ctx context.Context, params *dynamodb.PutItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error)
	DeleteItem(ctx context.Context, params *dynamodb.DeleteItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DeleteItemOutput, error)
	Query(ctx context.Context, params *dynamodb.QueryInput, optFns ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error)
	TransactWriteItems(ctx context.Context, params *dynamodb.TransactWriteItemsInput, optFns ...func(*dynamodb.Options)) (*dynamodb.TransactWriteItemsOutput, error)
}

var errDynamoDBSpaceStoreClosed = errors.New("dynamodb space store: store is closed")

// DynamoDBStore は Store + NonceStore の DynamoDB 実装。
//
// キー設計:
//   PK = USER#<userID>
//   SK = SPACE#<alias>  → SpaceRegistration
//   SK = PREF           → UserPreference
//   SK = NONCE#<nonce>  → NonceRecord（TTL 属性付き）
type DynamoDBStore struct {
	client    DynamoDBSpaceAPI
	tableName string
	mu        sync.RWMutex
	closed    bool
}

// NewDynamoDBStore は本番用コンストラクタ。AWS SDK デフォルト設定から DynamoDB クライアントを生成する。
func NewDynamoDBStore(tableName, region string) (*DynamoDBStore, error) {
	cfg, err := config.LoadDefaultConfig(context.Background(), config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("dynamodb space store: load config: %w", err)
	}
	client := dynamodb.NewFromConfig(cfg)
	return &DynamoDBStore{client: client, tableName: tableName}, nil
}

// NewDynamoDBStoreWithClient はテスト用コンストラクタ。DynamoDBSpaceAPI の任意実装を注入できる。
func NewDynamoDBStoreWithClient(tableName string, client DynamoDBSpaceAPI) *DynamoDBStore {
	return &DynamoDBStore{client: client, tableName: tableName}
}

func spacePK(userID string) string { return "USER#" + userID }
func spaceSK(alias string) string  { return "SPACE#" + alias }
func prefSK() string               { return "PREF" }
func nonceSK(nonce string) string  { return "NONCE#" + nonce }

// List は userID に紐付くすべての SpaceRegistration を返す。
func (s *DynamoDBStore) List(ctx context.Context, userID string) ([]SpaceRegistration, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return nil, errDynamoDBSpaceStoreClosed
	}

	pk := spacePK(userID)
	result := make([]SpaceRegistration, 0)
	var lastKey map[string]types.AttributeValue

	for {
		out, err := s.client.Query(ctx, &dynamodb.QueryInput{
			TableName:              &s.tableName,
			KeyConditionExpression: aws.String("pk = :pk AND begins_with(sk, :sk_prefix)"),
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":pk":        &types.AttributeValueMemberS{Value: pk},
				":sk_prefix": &types.AttributeValueMemberS{Value: "SPACE#"},
			},
			ExclusiveStartKey: lastKey,
		})
		if err != nil {
			return nil, fmt.Errorf("dynamodb space store: list: %w", err)
		}
		for _, item := range out.Items {
			reg, err := itemToSpaceRegistration(item)
			if err != nil {
				return nil, fmt.Errorf("dynamodb space store: list unmarshal: %w", err)
			}
			result = append(result, *reg)
		}
		if out.LastEvaluatedKey == nil {
			break
		}
		lastKey = out.LastEvaluatedKey
	}
	return result, nil
}

// Get は指定された userID + alias の SpaceRegistration を返す。存在しない場合は nil, nil。
func (s *DynamoDBStore) Get(ctx context.Context, userID, alias string) (*SpaceRegistration, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return nil, errDynamoDBSpaceStoreClosed
	}

	pk := spacePK(userID)
	sk := spaceSK(alias)
	out, err := s.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: &s.tableName,
		Key: map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: pk},
			"sk": &types.AttributeValueMemberS{Value: sk},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("dynamodb space store: get: %w", err)
	}
	if len(out.Item) == 0 {
		return nil, nil
	}
	return itemToSpaceRegistration(out.Item)
}

// Upsert は SpaceRegistration を保存する。同じキーが存在する場合は上書き。
func (s *DynamoDBStore) Upsert(ctx context.Context, reg *SpaceRegistration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return errDynamoDBSpaceStoreClosed
	}

	pk := spacePK(reg.UserID)
	sk := spaceSK(reg.Alias)
	item := spaceRegistrationToItem(pk, sk, reg)

	_, err := s.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: &s.tableName,
		Item:      item,
	})
	if err != nil {
		return fmt.Errorf("dynamodb space store: upsert: %w", err)
	}
	return nil
}

// Delete は指定された userID + alias のレコードを削除する。存在しない場合もエラーなし。
func (s *DynamoDBStore) Delete(ctx context.Context, userID, alias string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return errDynamoDBSpaceStoreClosed
	}

	pk := spacePK(userID)
	sk := spaceSK(alias)
	_, err := s.client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: &s.tableName,
		Key: map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: pk},
			"sk": &types.AttributeValueMemberS{Value: sk},
		},
	})
	if err != nil {
		return fmt.Errorf("dynamodb space store: delete: %w", err)
	}
	return nil
}

// GetPreference は userID の UserPreference を返す。存在しない場合は nil, nil。
func (s *DynamoDBStore) GetPreference(ctx context.Context, userID string) (*UserPreference, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return nil, errDynamoDBSpaceStoreClosed
	}

	pk := spacePK(userID)
	sk := prefSK()
	out, err := s.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: &s.tableName,
		Key: map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: pk},
			"sk": &types.AttributeValueMemberS{Value: sk},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("dynamodb space store: get preference: %w", err)
	}
	if len(out.Item) == 0 {
		return nil, nil
	}
	return itemToUserPreference(out.Item)
}

// PutPreference は UserPreference を保存する。
func (s *DynamoDBStore) PutPreference(ctx context.Context, pref *UserPreference) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return errDynamoDBSpaceStoreClosed
	}

	pk := spacePK(pref.UserID)
	sk := prefSK()
	item := userPreferenceToItem(pk, sk, pref)

	_, err := s.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: &s.tableName,
		Item:      item,
	})
	if err != nil {
		return fmt.Errorf("dynamodb space store: put preference: %w", err)
	}
	return nil
}

// Close はストアを閉じる。冪等。
func (s *DynamoDBStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	return nil
}

// Store は nonce を TTL 付きで保存する（NonceStore 実装）。
func (s *DynamoDBStore) Store(ctx context.Context, userID, nonce string, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return errDynamoDBSpaceStoreClosed
	}

	pk := spacePK(userID)
	sk := nonceSK(nonce)
	expiresAt := time.Now().Add(ttl).Unix()

	_, err := s.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: &s.tableName,
		Item: map[string]types.AttributeValue{
			"pk":         &types.AttributeValueMemberS{Value: pk},
			"sk":         &types.AttributeValueMemberS{Value: sk},
			"expires_at": &types.AttributeValueMemberN{Value: strconv.FormatInt(expiresAt, 10)},
		},
	})
	if err != nil {
		return fmt.Errorf("dynamodb space store: store nonce: %w", err)
	}
	return nil
}

// Consume は nonce を1回限り消費する（NonceStore 実装）。
// ConditionExpression で原子的に Delete し、存在しない場合は ErrNonceAlreadyUsed を返す。
func (s *DynamoDBStore) Consume(ctx context.Context, userID, nonce string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return errDynamoDBSpaceStoreClosed
	}

	pk := spacePK(userID)
	sk := nonceSK(nonce)

	_, err := s.client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: &s.tableName,
		Key: map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: pk},
			"sk": &types.AttributeValueMemberS{Value: sk},
		},
		ConditionExpression: aws.String("attribute_exists(pk)"),
	})
	if err != nil {
		var ccf *types.ConditionalCheckFailedException
		if errors.As(err, &ccf) {
			return ErrNonceAlreadyUsed
		}
		return fmt.Errorf("dynamodb space store: consume nonce: %w", err)
	}
	return nil
}

// --- 属性マップ変換ヘルパー ---

func spaceRegistrationToItem(pk, sk string, reg *SpaceRegistration) map[string]types.AttributeValue {
	return map[string]types.AttributeValue{
		"pk":               &types.AttributeValueMemberS{Value: pk},
		"sk":               &types.AttributeValueMemberS{Value: sk},
		"user_id":          &types.AttributeValueMemberS{Value: reg.UserID},
		"alias":            &types.AttributeValueMemberS{Value: reg.Alias},
		"tenant":           &types.AttributeValueMemberS{Value: reg.Tenant},
		"base_url":         &types.AttributeValueMemberS{Value: reg.BaseURL},
		"auth_type":        &types.AttributeValueMemberS{Value: string(reg.AuthType)},
		"auth_profile":     &types.AttributeValueMemberS{Value: reg.AuthProfile},
		"provider":         &types.AttributeValueMemberS{Value: reg.Provider},
		"status":           &types.AttributeValueMemberS{Value: string(reg.Status)},
		"last_verified_at": &types.AttributeValueMemberS{Value: reg.LastVerifiedAt.UTC().Format(time.RFC3339)},
		"created_at":       &types.AttributeValueMemberS{Value: reg.CreatedAt.UTC().Format(time.RFC3339)},
		"updated_at":       &types.AttributeValueMemberS{Value: reg.UpdatedAt.UTC().Format(time.RFC3339)},
		"disabled":         &types.AttributeValueMemberBOOL{Value: reg.Disabled},
	}
}

func itemToSpaceRegistration(item map[string]types.AttributeValue) (*SpaceRegistration, error) {
	reg := &SpaceRegistration{
		UserID:      getStrAttr(item, "user_id"),
		Alias:       getStrAttr(item, "alias"),
		Tenant:      getStrAttr(item, "tenant"),
		BaseURL:     getStrAttr(item, "base_url"),
		AuthType:    AuthType(getStrAttr(item, "auth_type")),
		AuthProfile: getStrAttr(item, "auth_profile"),
		Provider:    getStrAttr(item, "provider"),
		Status:      SpaceStatus(getStrAttr(item, "status")),
		Disabled:    getBoolAttr(item, "disabled"),
	}

	var err error
	reg.LastVerifiedAt, err = parseTimeAttr(item, "last_verified_at")
	if err != nil {
		return nil, fmt.Errorf("parse last_verified_at: %w", err)
	}
	reg.CreatedAt, err = parseTimeAttr(item, "created_at")
	if err != nil {
		return nil, fmt.Errorf("parse created_at: %w", err)
	}
	reg.UpdatedAt, err = parseTimeAttr(item, "updated_at")
	if err != nil {
		return nil, fmt.Errorf("parse updated_at: %w", err)
	}
	return reg, nil
}

func userPreferenceToItem(pk, sk string, pref *UserPreference) map[string]types.AttributeValue {
	return map[string]types.AttributeValue{
		"pk":                   &types.AttributeValueMemberS{Value: pk},
		"sk":                   &types.AttributeValueMemberS{Value: sk},
		"user_id":              &types.AttributeValueMemberS{Value: pref.UserID},
		"default_space_alias":  &types.AttributeValueMemberS{Value: pref.DefaultSpaceAlias},
		"created_at":           &types.AttributeValueMemberS{Value: pref.CreatedAt.UTC().Format(time.RFC3339)},
		"updated_at":           &types.AttributeValueMemberS{Value: pref.UpdatedAt.UTC().Format(time.RFC3339)},
	}
}

func itemToUserPreference(item map[string]types.AttributeValue) (*UserPreference, error) {
	pref := &UserPreference{
		UserID:            getStrAttr(item, "user_id"),
		DefaultSpaceAlias: getStrAttr(item, "default_space_alias"),
	}
	var err error
	pref.CreatedAt, err = parseTimeAttr(item, "created_at")
	if err != nil {
		return nil, fmt.Errorf("parse created_at: %w", err)
	}
	pref.UpdatedAt, err = parseTimeAttr(item, "updated_at")
	if err != nil {
		return nil, fmt.Errorf("parse updated_at: %w", err)
	}
	return pref, nil
}

func getStrAttr(item map[string]types.AttributeValue, key string) string {
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

func getBoolAttr(item map[string]types.AttributeValue, key string) bool {
	v, ok := item[key]
	if !ok {
		return false
	}
	bv, ok := v.(*types.AttributeValueMemberBOOL)
	if !ok {
		return false
	}
	return bv.Value
}

func parseTimeAttr(item map[string]types.AttributeValue, key string) (time.Time, error) {
	s := getStrAttr(item, key)
	if s == "" {
		return time.Time{}, nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse %s: %w", key, err)
	}
	return t.UTC(), nil
}
